#!/usr/bin/env python3
from pathlib import Path

path = Path("pkg/call/runtime/runtime_test.go")
text = path.read_text()
old = '''\tcall, _ = runtime.Call("call-1")
\tif call.State != StateActive {
\t\tt.Fatalf("expected active state, got %s", call.State)
\t}
'''
new = '''\tcall, _ = runtime.Call("call-1")
\tif call.State != StateConnecting {
\t\tt.Fatalf("expected connecting state before media, got %s", call.State)
\t}
'''
if text.count(old) != 1:
    raise RuntimeError("runtime lifecycle expectation did not match exactly once")
path.write_text(text.replace(old, new, 1))
Path("tools/fix_runtime_media_test.py").unlink()
