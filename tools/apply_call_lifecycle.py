#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: Path, old: str, new: str) -> None:
    text = path.read_text()
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"{path}: expected one match, found {count}: {old[:100]!r}")
    path.write_text(text.replace(old, new, 1))


whatsmeow = Path("pkg/whatsmeow/service/whatsmeow.go")
replace_once(
    whatsmeow,
    "type WhatsmeowService interface {\n",
    "type ClientLifecycle interface {\n"
    "\tAttachClient(instanceID string, client *whatsmeow.Client, prepareIncoming bool)\n"
    "\tDetachClient(instanceID string)\n"
    "}\n\n"
    "type WhatsmeowService interface {\n"
    "\tSetClientLifecycle(lifecycle ClientLifecycle)\n",
)
replace_once(
    whatsmeow,
    "\tloggerWrapper      *logger_wrapper.LoggerManager\n"
    "\tpasskeyCeremony    *ceremony.Store\n"
    "}",
    "\tloggerWrapper      *logger_wrapper.LoggerManager\n"
    "\tpasskeyCeremony    *ceremony.Store\n"
    "\tclientLifecycle    ClientLifecycle\n"
    "}",
)
replace_once(
    whatsmeow,
    "func (w whatsmeowService) ReconnectClient(instanceId string) error {\n",
    "func (w whatsmeowService) ReconnectClient(instanceId string) error {\n"
    "\tw.detachCallClient(instanceId)\n",
)
replace_once(
    whatsmeow,
    "\tif w.clientPointer[cd.Instance.Id] != nil {\n"
    "\t\tif w.clientPointer[cd.Instance.Id].IsConnected() {\n"
    "\t\t\treturn\n"
    "\t\t}\n"
    "\t}\n",
    "\tif existing := w.clientPointer[cd.Instance.Id]; existing != nil {\n"
    "\t\tif existing.IsConnected() {\n"
    "\t\t\treturn\n"
    "\t\t}\n"
    "\t\tw.detachCallClient(cd.Instance.Id)\n"
    "\t}\n",
)
replace_once(
    whatsmeow,
    "\t// Armazena o MyClient no map para permitir atualizações posteriores\n"
    "\tw.myClientPointer[cd.Instance.Id] = mycli\n",
    "\t// Armazena o MyClient no map para permitir atualizações posteriores\n"
    "\tw.myClientPointer[cd.Instance.Id] = mycli\n\n"
    "\t// Call monitoring starts with the WhatsApp client itself, before the\n"
    "\t// connection can emit an incoming offer. Auto-reject instances keep only\n"
    "\t// the public state tracker and do not decrypt/send preaccept.\n"
    "\tw.attachCallClient(cd.Instance.Id, client, !cd.Instance.RejectCall)\n",
)
replace_once(
    whatsmeow,
    "func (w whatsmeowService) ClearInstanceCache(instanceId string, token string) error {\n",
    "func (w whatsmeowService) ClearInstanceCache(instanceId string, token string) error {\n"
    "\tw.detachCallClient(instanceId)\n",
)

Path("pkg/whatsmeow/service/call_lifecycle.go").write_text(
    '''package whatsmeow_service

import "go.mau.fi/whatsmeow"

// SetClientLifecycle injects the call coordinator without coupling the
// WhatsApp service package to the call implementation.
func (w *whatsmeowService) SetClientLifecycle(lifecycle ClientLifecycle) {
\tw.clientLifecycle = lifecycle
}

func (w whatsmeowService) attachCallClient(instanceID string, client *whatsmeow.Client, prepareIncoming bool) {
\tif w.clientLifecycle != nil {
\t\tw.clientLifecycle.AttachClient(instanceID, client, prepareIncoming)
\t}
}

func (w whatsmeowService) detachCallClient(instanceID string) {
\tif w.clientLifecycle != nil {
\t\tw.clientLifecycle.DetachClient(instanceID)
\t}
}
'''
)

call_service = Path("pkg/call/service/call_service.go")
replace_once(
    call_service,
    "\tcall_runtime \"github.com/evolution-foundation/evolution-go/pkg/call/runtime\"\n"
    "\tcall_driver \"github.com/evolution-foundation/evolution-go/pkg/call/voip/driver\"\n"
    "\tcall_incoming \"github.com/evolution-foundation/evolution-go/pkg/call/voip/incoming\"\n",
    "\tcall_lifecycle \"github.com/evolution-foundation/evolution-go/pkg/call/lifecycle\"\n"
    "\tcall_runtime \"github.com/evolution-foundation/evolution-go/pkg/call/runtime\"\n"
    "\tcall_driver \"github.com/evolution-foundation/evolution-go/pkg/call/voip/driver\"\n",
)
replace_once(
    call_service,
    "\tloggerWrapper    *logger_wrapper.LoggerManager\n"
    "\truntimeRegistry  *call_runtime.Registry\n"
    "\tincomingRegistry *call_incoming.Registry\n",
    "\tloggerWrapper *logger_wrapper.LoggerManager\n"
    "\tcoordinator   *call_lifecycle.Coordinator\n",
)
replace_once(
    call_service,
    "\tc.runtimeRegistry.Attach(instanceID, client)\n"
    "\tc.incomingRegistry.Attach(instanceID, client)\n",
    "\tc.coordinator.Attach(instanceID, client)\n",
)
call_text = call_service.read_text()
call_text = call_text.replace("c.runtimeRegistry.Attach(", "c.coordinator.RuntimeFor(")
call_text = call_text.replace("c.incomingRegistry.Accept(", "c.coordinator.AcceptIncoming(")
call_text = call_text.replace("c.incomingRegistry.Terminate(", "c.coordinator.TerminateIncoming(")
call_text = call_text.replace("c.incomingRegistry.Remove(", "c.coordinator.RemoveIncoming(")
call_service.write_text(call_text)
replace_once(
    call_service,
    "func NewCallService(\n"
    "\tclientPointer map[string]*whatsmeow.Client,\n"
    "\twhatsmeowService whatsmeow_service.WhatsmeowService,\n"
    "\tloggerWrapper *logger_wrapper.LoggerManager,\n"
    ") CallService {\n"
    "\treturn &callService{\n"
    "\t\tclientPointer:    clientPointer,\n"
    "\t\twhatsmeowService: whatsmeowService,\n"
    "\t\tloggerWrapper:    loggerWrapper,\n"
    "\t\truntimeRegistry:  call_runtime.NewRegistry(),\n"
    "\t\tincomingRegistry: call_incoming.NewRegistry(),\n"
    "\t}\n"
    "}\n",
    "func NewCallService(\n"
    "\tclientPointer map[string]*whatsmeow.Client,\n"
    "\twhatsmeowService whatsmeow_service.WhatsmeowService,\n"
    "\tloggerWrapper *logger_wrapper.LoggerManager,\n"
    "\tcoordinator *call_lifecycle.Coordinator,\n"
    ") CallService {\n"
    "\tif coordinator == nil {\n"
    "\t\tcoordinator = call_lifecycle.NewCoordinator()\n"
    "\t}\n"
    "\treturn &callService{\n"
    "\t\tclientPointer:    clientPointer,\n"
    "\t\twhatsmeowService: whatsmeowService,\n"
    "\t\tloggerWrapper:    loggerWrapper,\n"
    "\t\tcoordinator:      coordinator,\n"
    "\t}\n"
    "}\n",
)

main = Path("cmd/evolution-go/main.go")
replace_once(
    main,
    "\tcall_handler \"github.com/evolution-foundation/evolution-go/pkg/call/handler\"\n"
    "\tcall_service \"github.com/evolution-foundation/evolution-go/pkg/call/service\"\n",
    "\tcall_handler \"github.com/evolution-foundation/evolution-go/pkg/call/handler\"\n"
    "\tcall_lifecycle \"github.com/evolution-foundation/evolution-go/pkg/call/lifecycle\"\n"
    "\tcall_service \"github.com/evolution-foundation/evolution-go/pkg/call/service\"\n",
)
replace_once(
    main,
    "\tinstanceService := instance_service.NewInstanceService(\n",
    "\tcallCoordinator := call_lifecycle.NewCoordinator()\n"
    "\twhatsmeowService.SetClientLifecycle(callCoordinator)\n\n"
    "\tinstanceService := instance_service.NewInstanceService(\n",
)
replace_once(
    main,
    "\tcallService := call_service.NewCallService(clientPointer, whatsmeowService, loggerWrapper)\n",
    "\tcallService := call_service.NewCallService(clientPointer, whatsmeowService, loggerWrapper, callCoordinator)\n",
)

Path("tools/apply_call_lifecycle.py").unlink()
Path(".github/workflows/apply-call-lifecycle.yml").unlink()
