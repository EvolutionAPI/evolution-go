## Summary

WhatsApp renders the **Encaminhada** badge when `ContextInfo.ForwardingScore > 0`. Evolution Go v1 did not expose this field, so forwarded messages arrived without the badge in the recipient's WhatsApp.

## Changes

- `TextStruct` and `MediaStruct` accept optional `forwardingScore` (uint32)
- `SendDataStruct` carries it through to `SendMessage`
- `SendMessage` applies it to the `ContextInfo` of every message type (text, image, video, ptv, audio, document, poll, sticker, location, contact, interactive, list)

## Usage

```json
POST /send/text
{
  "number": "5511999999999",
  "text": "forwarded message",
  "forwardingScore": 1
}
```

When `forwardingScore > 0`, the recipient's WhatsApp shows the **Encaminhada** badge.