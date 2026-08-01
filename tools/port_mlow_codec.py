#!/usr/bin/env python3
from __future__ import annotations

import shutil
import subprocess
import tempfile
from pathlib import Path

UPSTREAM_URL = "https://github.com/JotaDev66/WaCalls.git"
UPSTREAM_COMMIT = "edeb31f0427aba896639db503153b777a405eccf"
ROOT = Path(__file__).resolve().parents[1]
TARGET = ROOT / "pkg/call/voip/media/mlow"
LICENSE_HEADER = "// Portions are derived from JotaDev66/WaCalls under the MIT license in ../../LICENSE-WACALLS.\n"


def run(*args: str, cwd: Path | None = None) -> None:
    subprocess.run(args, cwd=cwd, check=True)


def add_license_header(source: str) -> str:
    if "JotaDev66/WaCalls" in source[:300]:
        return source
    return LICENSE_HEADER + source


with tempfile.TemporaryDirectory(prefix="wacalls-mlow-") as tmp:
    checkout = Path(tmp) / "WaCalls"
    run("git", "clone", "--filter=blob:none", "--no-checkout", UPSTREAM_URL, str(checkout))
    run("git", "-C", str(checkout), "fetch", "--depth", "1", "origin", UPSTREAM_COMMIT)
    run("git", "-C", str(checkout), "checkout", "--detach", UPSTREAM_COMMIT)

    source_dir = checkout / "internal/voip/media/mlow"
    sources = sorted(path for path in source_dir.glob("*.go") if not path.name.endswith("_test.go"))
    assets = sorted(source_dir.glob("*.bin"))
    if len(sources) < 20:
        raise RuntimeError(f"expected the complete MLow implementation, found only {len(sources)} files")
    if len(assets) < 3:
        raise RuntimeError(f"expected embedded MLow tables, found only {len(assets)} binary assets")

    shutil.rmtree(TARGET, ignore_errors=True)
    TARGET.mkdir(parents=True, exist_ok=True)
    for source_path in sources:
        source = source_path.read_text(encoding="utf-8")
        if "package mlow" not in source:
            raise RuntimeError(f"unexpected package in {source_path}")
        (TARGET / source_path.name).write_text(add_license_header(source), encoding="utf-8")
    for asset_path in assets:
        shutil.copy2(asset_path, TARGET / asset_path.name)

codec = '''// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import "errors"

const (
    MLowSampleRate = 16000
    MLowFrameSize = 960
)

var (
    ErrCodecClosed = errors.New("audio codec is closed")
    ErrInvalidPCMFrame = errors.New("invalid PCM frame size")
)

type Codec interface {
    Encode(pcm []float32) ([]byte, error)
    Decode(frame []byte) ([]float32, error)
    FrameSize() int
    SampleRate() int
    Close()
}

type CodecOptions struct {
    Bitrate int
    Complexity int
    FEC bool
}

var DefaultCodecOptions = CodecOptions{Bitrate: 6000, Complexity: 5, FEC: false}

func NormalizeFrame(pcm []float32, samples int) []float32 {
    if samples <= 0 {
        return nil
    }
    if len(pcm) == samples {
        return append([]float32(nil), pcm...)
    }
    normalized := make([]float32, samples)
    copy(normalized, pcm)
    return normalized
}
'''
(ROOT / "pkg/call/voip/media/codec.go").write_text(codec, encoding="utf-8")

adapter = '''// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package media

import (
    "fmt"
    "sync"

    "github.com/evolution-foundation/evolution-go/pkg/call/voip/media/mlow"
)

type mlowCodec struct {
    mu sync.Mutex
    enc *mlow.MlowEncoder
    dec *mlow.MlowDecoder
    closed bool
}

func NewMLowCodec(opts CodecOptions) (Codec, error) {
    _ = opts
    return &mlowCodec{enc: mlow.NewMlowEncoder(), dec: mlow.NewMlowDecoder()}, nil
}

func (c *mlowCodec) Encode(pcm []float32) ([]byte, error) {
    if len(pcm) == 0 {
        return nil, nil
    }
    if len(pcm) != MLowFrameSize {
        return nil, fmt.Errorf("%w: got %d samples, want %d", ErrInvalidPCMFrame, len(pcm), MLowFrameSize)
    }
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.closed || c.enc == nil {
        return nil, ErrCodecClosed
    }
    input := append([]float32(nil), pcm...)
    defer zeroFloat32(input)
    encoded, err := c.enc.Encode(input)
    if err != nil {
        return nil, fmt.Errorf("encode MLow frame: %w", err)
    }
    return append([]byte(nil), encoded...), nil
}

func (c *mlowCodec) Decode(frame []byte) ([]float32, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.closed || c.dec == nil {
        return nil, ErrCodecClosed
    }
    input := append([]byte(nil), frame...)
    defer zeroBytes(input)
    decoded := c.dec.Decode(input)
    return NormalizeFrame(decoded, MLowFrameSize), nil
}

func (c *mlowCodec) FrameSize() int { return MLowFrameSize }
func (c *mlowCodec) SampleRate() int { return MLowSampleRate }

func (c *mlowCodec) Close() {
    if c == nil {
        return
    }
    c.mu.Lock()
    c.closed = true
    c.enc = nil
    c.dec = nil
    c.mu.Unlock()
}

func zeroFloat32(values []float32) {
    for index := range values {
        values[index] = 0
    }
}
'''
(ROOT / "pkg/call/voip/media/mlow_codec.go").write_text(adapter, encoding="utf-8")

test = '''package media

import (
    "errors"
    "math"
    "sync"
    "testing"
)

func TestMLowCodecAdapterRoundtrip(t *testing.T) {
    codec, err := NewMLowCodec(DefaultCodecOptions)
    if err != nil {
        t.Fatal(err)
    }
    defer codec.Close()
    frame := make([]float32, MLowFrameSize)
    for i := range frame {
        frame[i] = 0.25 * float32(math.Sin(2*math.Pi*440*float64(i)/MLowSampleRate))
    }
    encoded, err := codec.Encode(frame)
    if err != nil {
        t.Fatal(err)
    }
    if len(encoded) == 0 {
        t.Fatal("encoded frame is empty")
    }
    decoded, err := codec.Decode(encoded)
    if err != nil {
        t.Fatal(err)
    }
    if len(decoded) != MLowFrameSize {
        t.Fatalf("decoded %d samples, want %d", len(decoded), MLowFrameSize)
    }
}

func TestMLowCodecPLCAndValidation(t *testing.T) {
    codec, err := NewMLowCodec(DefaultCodecOptions)
    if err != nil {
        t.Fatal(err)
    }
    plc, err := codec.Decode(nil)
    if err != nil {
        t.Fatal(err)
    }
    if len(plc) != MLowFrameSize {
        t.Fatalf("PLC returned %d samples", len(plc))
    }
    if _, err = codec.Encode(make([]float32, 12)); !errors.Is(err, ErrInvalidPCMFrame) {
        t.Fatalf("expected ErrInvalidPCMFrame, got %v", err)
    }
    codec.Close()
    if _, err = codec.Decode(nil); !errors.Is(err, ErrCodecClosed) {
        t.Fatalf("expected ErrCodecClosed, got %v", err)
    }
}

func TestMLowCodecSerializesConcurrentUse(t *testing.T) {
    codec, err := NewMLowCodec(DefaultCodecOptions)
    if err != nil {
        t.Fatal(err)
    }
    defer codec.Close()
    frame := make([]float32, MLowFrameSize)
    var wg sync.WaitGroup
    for worker := 0; worker < 4; worker++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for attempt := 0; attempt < 4; attempt++ {
                encoded, encodeErr := codec.Encode(frame)
                if encodeErr != nil {
                    t.Errorf("encode: %v", encodeErr)
                    return
                }
                if _, decodeErr := codec.Decode(encoded); decodeErr != nil {
                    t.Errorf("decode: %v", decodeErr)
                    return
                }
            }
        }()
    }
    wg.Wait()
}
'''
(ROOT / "pkg/call/voip/media/mlow_codec_test.go").write_text(test, encoding="utf-8")

(ROOT / "tools/port_mlow_codec.py").unlink()
(ROOT / ".github/workflows/port-mlow-codec.yml").unlink()
