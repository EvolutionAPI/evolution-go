#!/usr/bin/env python3
from pathlib import Path


def replace_exact(path: Path, old: str, new: str, expected: int = 1) -> None:
    text = path.read_text()
    count = text.count(old)
    if count != expected:
        raise RuntimeError(f"{path}: expected {expected} matches, found {count}: {old[:100]!r}")
    path.write_text(text.replace(old, new, expected))


relay = Path("pkg/call/voip/media/relay_registry.go")
replace_exact(relay, "\t\trelay.SetOnConnected(func(_, _ int) {})\n", "")
replace_exact(
    relay,
    "\t\tgo s.start(event.CallID)\n",
    "\t\tgo s.startLogged(event.CallID)\n",
    expected=2,
)
replace_exact(
    relay,
    "func (s *relaySession) start(callID string) error {\n",
    "func (s *relaySession) startLogged(callID string) {\n"
    "\tif err := s.start(callID); err != nil {\n"
    "\t\ts.log.Warn(\"WhatsApp relay setup failed\", \"instance\", s.instanceID, \"call_id\", callID, \"err\", err)\n"
    "\t}\n"
    "}\n\n"
    "func (s *relaySession) start(callID string) error {\n",
)
replace_exact(
    relay,
    "\t\tif s.onConnected != nil {\n"
    "\t\t\ts.onConnected(s.instanceID, callID)\n"
    "\t\t}\n",
    "\t\ts.mu.Lock()\n"
    "\t\tcallback := s.onConnected\n"
    "\t\ts.mu.Unlock()\n"
    "\t\tif callback != nil {\n"
    "\t\t\tcallback(s.instanceID, callID)\n"
    "\t\t}\n",
)
replace_exact(
    relay,
    "\trelay.SetOnReceive(func(packet []byte) {\n"
    "\t\tif s.onPacket != nil {\n"
    "\t\t\ts.onPacket(s.instanceID, callID, append([]byte(nil), packet...))\n"
    "\t\t}\n"
    "\t})\n",
    "\trelay.SetOnReceive(func(packet []byte) {\n"
    "\t\ts.mu.Lock()\n"
    "\t\tcallback := s.onPacket\n"
    "\t\ts.mu.Unlock()\n"
    "\t\tif callback != nil {\n"
    "\t\t\tcallback(s.instanceID, callID, append([]byte(nil), packet...))\n"
    "\t\t}\n"
    "\t})\n",
)
replace_exact(
    relay,
    "func (r *RelayRegistry) Start(instanceID, callID string) error {\n"
    "\tr.mu.RLock()\n"
    "\tsession := r.sessions[instanceID]\n"
    "\tr.mu.RUnlock()\n"
    "\tif session == nil {\n"
    "\t\treturn fmt.Errorf(\"relay runtime is not attached for instance %s\", instanceID)\n"
    "\t}\n"
    "\treturn session.start(callID)\n"
    "}\n",
    "func (r *RelayRegistry) Start(instanceID, callID string) error {\n"
    "\tr.mu.RLock()\n"
    "\tsession := r.sessions[instanceID]\n"
    "\tr.mu.RUnlock()\n"
    "\tif session == nil {\n"
    "\t\treturn fmt.Errorf(\"relay runtime is not attached for instance %s\", instanceID)\n"
    "\t}\n"
    "\terr := session.start(callID)\n"
    "\tif err != nil {\n"
    "\t\tr.log.Warn(\"WhatsApp relay setup failed\", \"instance\", instanceID, \"call_id\", callID, \"err\", err)\n"
    "\t}\n"
    "\treturn err\n"
    "}\n",
)

coordinator = Path("pkg/call/lifecycle/coordinator.go")
replace_exact(
    coordinator,
    "func (c *Coordinator) AcceptIncoming(ctx context.Context, instanceID, callID string) error {\n"
    "\tif err := c.incoming.Accept(ctx, instanceID, callID); err != nil {\n"
    "\t\treturn err\n"
    "\t}\n"
    "\treturn c.relays.Start(instanceID, callID)\n"
    "}\n",
    "func (c *Coordinator) AcceptIncoming(ctx context.Context, instanceID, callID string) error {\n"
    "\tif err := c.incoming.Accept(ctx, instanceID, callID); err != nil {\n"
    "\t\treturn err\n"
    "\t}\n"
    "\tgo func() { _ = c.relays.Start(instanceID, callID) }()\n"
    "\treturn nil\n"
    "}\n",
)

runtime = Path("pkg/call/runtime/runtime.go")
replace_exact(
    runtime,
    "\tcase *events.CallAccept:\n"
    "\t\tr.Transition(\n"
    "\t\t\tevent.CallID,\n"
    "\t\t\tcallPeer(event.CallCreator, event.From),\n"
    "\t\t\tDirectionOutgoing,\n"
    "\t\t\tStateActive,\n"
    "\t\t\tnil,\n"
    "\t\t\t\"\",\n"
    "\t\t)\n",
    "\tcase *events.CallAccept:\n"
    "\t\tr.Transition(\n"
    "\t\t\tevent.CallID,\n"
    "\t\t\tcallPeer(event.CallCreator, event.From),\n"
    "\t\t\tDirectionOutgoing,\n"
    "\t\t\tStateConnecting,\n"
    "\t\t\tnil,\n"
    "\t\t\t\"\",\n"
    "\t\t)\n",
)

Path("tools/apply_media_orchestration.py").unlink()
Path(".github/workflows/apply-media-orchestration.yml").unlink()
