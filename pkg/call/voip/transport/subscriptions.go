// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package transport

func encodeVarint(value uint64) []byte {
	var output []byte
	for value > 0x7f {
		output = append(output, byte((value&0x7f)|0x80))
		value >>= 7
	}
	return append(output, byte(value&0x7f))
}

func encodeProtobufVarintField(fieldNumber int, value uint64) []byte {
	tag := encodeVarint(uint64(fieldNumber << 3))
	return append(tag, encodeVarint(value)...)
}

func encodeProtobufLengthDelimited(fieldNumber int, data []byte) []byte {
	tag := encodeVarint(uint64((fieldNumber << 3) | 2))
	output := append(tag, encodeVarint(uint64(len(data)))...)
	return append(output, data...)
}

// BuildSenderSubscriptions creates the WhatsApp relay subscription protobuf
// attached to STUN binding requests.
func BuildSenderSubscriptions(ssrc uint32) []byte {
	inner := concat(
		encodeProtobufVarintField(3, uint64(ssrc)),
		encodeProtobufVarintField(5, 0),
		encodeProtobufVarintField(6, 0),
	)
	return encodeProtobufLengthDelimited(1, inner)
}

// BuildSSRCSubscriptionList creates the allocation payload for local and remote
// media SSRCs. Zero SSRC values are omitted.
func BuildSSRCSubscriptionList(selfSSRCs, peerSSRCs []uint32, selfPID, peerPID int) []byte {
	var entries [][]byte
	for _, ssrc := range selfSSRCs {
		if ssrc == 0 {
			continue
		}
		inner := concat(
			encodeProtobufVarintField(1, uint64(selfPID)),
			encodeProtobufVarintField(2, 1),
			encodeProtobufVarintField(3, uint64(ssrc)),
		)
		entries = append(entries, encodeProtobufLengthDelimited(1, inner))
	}
	for _, ssrc := range peerSSRCs {
		if ssrc == 0 {
			continue
		}
		inner := concat(
			encodeProtobufVarintField(1, uint64(peerPID)),
			encodeProtobufVarintField(2, 1),
			encodeProtobufVarintField(3, uint64(ssrc)),
		)
		entries = append(entries, encodeProtobufLengthDelimited(1, inner))
	}
	return concat(entries...)
}

func concat(parts ...[]byte) []byte {
	var output []byte
	for _, part := range parts {
		output = append(output, part...)
	}
	return output
}
