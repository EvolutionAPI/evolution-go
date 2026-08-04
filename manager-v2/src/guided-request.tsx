import { useEffect, useState, type ReactNode } from "react";
import type { BodyMode } from "./api-catalog";
import "./guided-request.css";

type JsonObject = Record<string, unknown>;

type GuidedRequestProps = {
  operationId: string;
  body: string;
  onChange: (body: string) => void;
};

export type RequestValidation = {
  errors: string[];
  warnings: string[];
};

const GUIDED_OPERATIONS = new Set([
  "send-text",
  "send-link",
  "send-media-file",
  "send-media-url",
  "send-poll",
  "send-sticker",
  "send-location",
  "send-contact",
  "send-button",
  "send-list",
  "send-carousel",
  "send-status-text",
  "send-status-media-file",
  "send-status-media-url",
]);

function parseObject(body: string): { value: JsonObject | null; error: string } {
  try {
    const parsed = JSON.parse(body || "{}") as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return { value: null, error: "O corpo precisa ser um objeto JSON." };
    }
    return { value: parsed as JsonObject, error: "" };
  } catch (cause) {
    return { value: null, error: cause instanceof Error ? cause.message : "JSON inválido" };
  }
}

function objectValue(value: unknown): JsonObject {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonObject : {};
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : value === undefined || value === null ? "" : String(value);
}

function numberValue(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : Number(value) || 0;
}

function booleanValue(value: unknown): boolean {
  return value === true;
}

function splitLines(value: string): string[] {
  return value.split("\n").map((item) => item.trim()).filter(Boolean);
}

function rowsToText(rows: unknown): string {
  return arrayValue(rows).map((row) => {
    const item = objectValue(row);
    return [stringValue(item.title), stringValue(item.description), stringValue(item.rowId)].join("|");
  }).join("\n");
}

function textToRows(value: string): JsonObject[] {
  return splitLines(value).map((line, index) => {
    const [title = "", description = "", rowId = ""] = line.split("|").map((item) => item.trim());
    return { title, description, rowId: rowId || `row_${index + 1}` };
  });
}

function buttonPreset(buttonsValue: unknown): "reply" | "cta" | "pix" {
  const buttons = arrayValue(buttonsValue).map(objectValue);
  if (buttons.some((item) => stringValue(item.type).toLowerCase() === "pix")) return "pix";
  if (buttons.some((item) => ["copy", "url", "call"].includes(stringValue(item.type).toLowerCase()))) return "cta";
  return "reply";
}

function presetButtons(preset: "reply" | "cta" | "pix"): JsonObject[] {
  if (preset === "pix") {
    return [{ type: "pix", currency: "BRL", name: "Minha empresa", keyType: "random", key: "CHAVE_PIX" }];
  }
  if (preset === "cta") {
    return [
      { type: "copy", displayText: "Copiar cupom", id: "copy_coupon", copyCode: "PROMO2026" },
      { type: "url", displayText: "Abrir site", url: "https://example.com" },
      { type: "call", displayText: "Ligar", phoneNumber: "+5562999999999" },
    ];
  }
  return [
    { type: "reply", displayText: "Quero saber mais", id: "btn_info" },
    { type: "reply", displayText: "Agora não", id: "btn_no" },
  ];
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="guided-field">
      <span>{label}</span>
      {children}
      {hint && <small>{hint}</small>}
    </label>
  );
}

function TextField({ label, value, onChange, placeholder, hint, type = "text" }: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  hint?: string;
  type?: "text" | "url";
}) {
  return <Field label={label} hint={hint}><input type={type} value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} /></Field>;
}

function NumberField({ label, value, onChange, step }: { label: string; value: number; onChange: (value: number) => void; step?: string }) {
  return <Field label={label}><input type="number" step={step} value={value} onChange={(event) => onChange(Number(event.target.value))} /></Field>;
}

function TextAreaField({ label, value, onChange, placeholder, hint, rows = 4 }: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  hint?: string;
  rows?: number;
}) {
  return <Field label={label} hint={hint}><textarea value={value} placeholder={placeholder} rows={rows} onChange={(event) => onChange(event.target.value)} /></Field>;
}

function JsonArrayField({ label, value, onChange, hint, rows = 9 }: {
  label: string;
  value: unknown;
  onChange: (value: unknown[]) => void;
  hint?: string;
  rows?: number;
}) {
  const serialized = JSON.stringify(arrayValue(value), null, 2);
  const [draft, setDraft] = useState(serialized);
  const [draftError, setDraftError] = useState("");

  useEffect(() => {
    setDraft(serialized);
    setDraftError("");
  }, [serialized]);

  const updateDraft = (next: string) => {
    setDraft(next);
    try {
      const parsed = JSON.parse(next) as unknown;
      if (!Array.isArray(parsed)) throw new Error("Informe um array JSON.");
      onChange(parsed);
      setDraftError("");
    } catch (cause) {
      setDraftError(cause instanceof Error ? cause.message : "JSON inválido");
    }
  };

  return (
    <Field label={label} hint={hint}>
      <textarea value={draft} rows={rows} spellCheck={false} onChange={(event) => updateDraft(event.target.value)} />
      {draftError && <span className="guided-json-error">Rascunho ainda inválido: {draftError}</span>}
    </Field>
  );
}

function SelectField({ label, value, onChange, options }: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
}) {
  return <Field label={label}><select value={value} onChange={(event) => onChange(event.target.value)}>{options.map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></Field>;
}

function CommonFields({ data, patch }: { data: JsonObject; patch: (key: string, value: unknown) => void }) {
  return (
    <div className="guided-grid compact">
      <NumberField label="Atraso (ms)" value={numberValue(data.delay)} onChange={(value) => patch("delay", value)} />
      <Field label="Formatação do JID"><span className="guided-check"><input type="checkbox" checked={data.formatJid !== false} onChange={(event) => patch("formatJid", event.target.checked)} /><span>Formatar automaticamente</span></span></Field>
      <Field label="Menções"><span className="guided-check"><input type="checkbox" checked={booleanValue(data.mentionAll)} onChange={(event) => patch("mentionAll", event.target.checked)} /><span>Mencionar todos</span></span></Field>
    </div>
  );
}

function Preview({ operationId, data }: { operationId: string; data: JsonObject }) {
  const type = stringValue(data.type) || "mensagem";
  let title = stringValue(data.title) || stringValue(data.question) || stringValue(data.name) || "Prévia da mensagem";
  let content = stringValue(data.text) || stringValue(data.description) || stringValue(data.caption) || stringValue(data.address);
  let details: ReactNode = null;

  if (operationId === "send-poll") {
    details = <ul>{arrayValue(data.options).map((item, index) => <li key={`${stringValue(item)}-${index}`}>{stringValue(item)}</li>)}</ul>;
  } else if (operationId === "send-button") {
    details = <div className="preview-buttons">{arrayValue(data.buttons).map((item, index) => {
      const button = objectValue(item);
      return <span key={index}>{stringValue(button.displayText) || stringValue(button.name) || stringValue(button.type)}</span>;
    })}</div>;
  } else if (operationId === "send-list") {
    const first = objectValue(arrayValue(data.sections)[0]);
    details = <ul>{arrayValue(first.rows).map((item, index) => <li key={index}>{stringValue(objectValue(item).title)}</li>)}</ul>;
  } else if (operationId === "send-carousel") {
    const card = objectValue(arrayValue(data.cards)[0]);
    const header = objectValue(card.header);
    title = stringValue(header.title) || title;
    content = stringValue(objectValue(card.body).text) || content;
    details = <span className="preview-chip">1º card do carrossel</span>;
  } else if (["send-media-file", "send-media-url", "send-status-media-file", "send-status-media-url"].includes(operationId)) {
    details = <span className="preview-chip">{type}</span>;
  } else if (operationId === "send-location") {
    details = <span className="preview-chip">📍 {numberValue(data.latitude)}, {numberValue(data.longitude)}</span>;
  } else if (operationId === "send-contact") {
    const vcard = objectValue(data.vcard);
    title = stringValue(vcard.fullName) || "Contato";
    content = stringValue(vcard.phone);
    details = <span className="preview-chip">{stringValue(vcard.organization) || "vCard"}</span>;
  }

  return (
    <aside className="guided-preview">
      <span className="eyebrow">Prévia aproximada</span>
      <div className="preview-phone">
        <div className="preview-header">WhatsApp</div>
        <div className="preview-bubble">
          <strong>{title}</strong>
          {content && <p>{content}</p>}
          {details}
          <small>{new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</small>
        </div>
      </div>
      <p>A renderização final depende do cliente WhatsApp e do tipo de mensagem.</p>
    </aside>
  );
}

export function supportsGuidedRequest(operationId: string): boolean {
  return GUIDED_OPERATIONS.has(operationId);
}

export function GuidedRequestEditor({ operationId, body, onChange }: GuidedRequestProps) {
  const parsed = parseObject(body);
  if (!parsed.value) {
    return <div className="guided-invalid"><strong>Não foi possível abrir o formulário guiado.</strong><span>{parsed.error}</span></div>;
  }
  const data = parsed.value;
  const commit = (next: JsonObject) => onChange(JSON.stringify(next, null, 2));
  const patch = (key: string, value: unknown) => commit({ ...data, [key]: value });
  const patchNested = (parent: string, key: string, value: unknown) => commit({ ...data, [parent]: { ...objectValue(data[parent]), [key]: value } });
  const patchObjectArrayItem = (key: string, index: number, update: (current: JsonObject) => JsonObject) => {
    const items = arrayValue(data[key]).map(objectValue);
    while (items.length <= index) items.push({});
    const next = [...items];
    next[index] = update(next[index]);
    patch(key, next);
  };
  const numberField = !operationId.startsWith("send-status");

  let fields: ReactNode;
  switch (operationId) {
    case "send-text":
      fields = <TextAreaField label="Texto" value={stringValue(data.text)} onChange={(value) => patch("text", value)} placeholder="Digite a mensagem de teste" rows={7} />;
      break;
    case "send-link":
      fields = <>
        <TextAreaField label="Texto" value={stringValue(data.text)} onChange={(value) => patch("text", value)} />
        <div className="guided-grid"><TextField label="URL" type="url" value={stringValue(data.url)} onChange={(value) => patch("url", value)} /><TextField label="Título" value={stringValue(data.title)} onChange={(value) => patch("title", value)} /><TextField label="Imagem da prévia" type="url" value={stringValue(data.imgUrl)} onChange={(value) => patch("imgUrl", value)} /></div>
        <TextAreaField label="Descrição" value={stringValue(data.description)} onChange={(value) => patch("description", value)} />
      </>;
      break;
    case "send-media-file":
    case "send-media-url":
    case "send-status-media-file":
    case "send-status-media-url":
      fields = <>
        <div className="guided-grid">
          <SelectField label="Tipo" value={stringValue(data.type) || "image"} onChange={(value) => patch("type", value)} options={[{ value: "image", label: "Imagem" }, { value: "video", label: "Vídeo" }, { value: "audio", label: "Áudio" }, { value: "document", label: "Documento" }]} />
          {operationId.endsWith("url") && <TextField label="URL pública ou base64" value={stringValue(data.url)} onChange={(value) => patch("url", value)} />}
          {!operationId.startsWith("send-status") && <TextField label="Nome do arquivo" value={stringValue(data.filename)} onChange={(value) => patch("filename", value)} />}
        </div>
        <TextAreaField label="Legenda" value={stringValue(data.caption)} onChange={(value) => patch("caption", value)} />
      </>;
      break;
    case "send-poll":
      fields = <>
        <TextField label="Pergunta" value={stringValue(data.question)} onChange={(value) => patch("question", value)} />
        <TextAreaField label="Opções" value={arrayValue(data.options).map(stringValue).join("\n")} onChange={(value) => patch("options", splitLines(value))} hint="Uma opção por linha; mínimo de duas." />
        <NumberField label="Máximo de respostas" value={numberValue(data.maxAnswer) || 1} onChange={(value) => patch("maxAnswer", value)} />
      </>;
      break;
    case "send-sticker":
      fields = <TextField label="URL ou base64 da figurinha" value={stringValue(data.sticker)} onChange={(value) => patch("sticker", value)} />;
      break;
    case "send-location":
      fields = <>
        <div className="guided-grid"><TextField label="Nome do local" value={stringValue(data.name)} onChange={(value) => patch("name", value)} /><TextField label="Endereço" value={stringValue(data.address)} onChange={(value) => patch("address", value)} /></div>
        <div className="guided-grid"><NumberField label="Latitude" step="any" value={numberValue(data.latitude)} onChange={(value) => patch("latitude", value)} /><NumberField label="Longitude" step="any" value={numberValue(data.longitude)} onChange={(value) => patch("longitude", value)} /></div>
      </>;
      break;
    case "send-contact": {
      const vcard = objectValue(data.vcard);
      fields = <div className="guided-grid"><TextField label="Nome completo" value={stringValue(vcard.fullName)} onChange={(value) => patchNested("vcard", "fullName", value)} /><TextField label="Telefone do contato" value={stringValue(vcard.phone)} onChange={(value) => patchNested("vcard", "phone", value)} /><TextField label="Organização" value={stringValue(vcard.organization)} onChange={(value) => patchNested("vcard", "organization", value)} /></div>;
      break;
    }
    case "send-button": {
      const preset = buttonPreset(data.buttons);
      fields = <>
        <div className="guided-grid"><TextField label="Título" value={stringValue(data.title)} onChange={(value) => patch("title", value)} /><TextField label="Rodapé" value={stringValue(data.footer)} onChange={(value) => patch("footer", value)} /></div>
        <TextAreaField label="Descrição" value={stringValue(data.description)} onChange={(value) => patch("description", value)} />
        <SelectField label="Combinação segura" value={preset} onChange={(value) => patch("buttons", presetButtons(value as "reply" | "cta" | "pix"))} options={[{ value: "reply", label: "Até 3 respostas rápidas" }, { value: "cta", label: "Copiar + URL + ligar" }, { value: "pix", label: "PIX isolado" }]} />
        <JsonArrayField label="Botões em JSON" value={data.buttons} onChange={(value) => patch("buttons", value)} hint="O rascunho pode ficar temporariamente inválido sem perder o texto digitado." />
      </>;
      break;
    }
    case "send-list": {
      const section = objectValue(arrayValue(data.sections)[0]);
      fields = <>
        <div className="guided-grid"><TextField label="Título" value={stringValue(data.title)} onChange={(value) => patch("title", value)} /><TextField label="Texto do botão" value={stringValue(data.buttonText)} onChange={(value) => patch("buttonText", value)} /></div>
        <TextAreaField label="Descrição" value={stringValue(data.description)} onChange={(value) => patch("description", value)} />
        <TextField label="Título da seção" value={stringValue(section.title)} onChange={(value) => patchObjectArrayItem("sections", 0, (current) => ({ ...current, title: value }))} />
        <TextAreaField label="Linhas da lista" rows={7} value={rowsToText(section.rows)} onChange={(value) => patchObjectArrayItem("sections", 0, (current) => ({ ...current, rows: textToRows(value) }))} hint="Uma linha por item: Título|Descrição|rowId" />
      </>;
      break;
    }
    case "send-carousel": {
      const card = objectValue(arrayValue(data.cards)[0]);
      const header = objectValue(card.header);
      const cardBody = objectValue(card.body);
      fields = <>
        <TextAreaField label="Texto principal" value={stringValue(data.body)} onChange={(value) => patch("body", value)} />
        <div className="guided-grid"><TextField label="Título do primeiro card" value={stringValue(header.title)} onChange={(value) => patchObjectArrayItem("cards", 0, (current) => ({ ...current, header: { ...objectValue(current.header), title: value } }))} /><TextField label="Imagem" value={stringValue(header.imageUrl)} onChange={(value) => patchObjectArrayItem("cards", 0, (current) => ({ ...current, header: { ...objectValue(current.header), imageUrl: value } }))} /></div>
        <TextAreaField label="Texto do card" value={stringValue(cardBody.text)} onChange={(value) => patchObjectArrayItem("cards", 0, (current) => ({ ...current, body: { ...objectValue(current.body), text: value } }))} />
      </>;
      break;
    }
    case "send-status-text":
      fields = <TextAreaField label="Texto do status" value={stringValue(data.text)} onChange={(value) => patch("text", value)} rows={7} />;
      break;
    default:
      fields = null;
  }

  return (
    <div className="guided-request-layout">
      <div className="guided-form">
        {numberField && <TextField label="Destinatário" value={stringValue(data.number)} onChange={(value) => patch("number", value)} placeholder="5562999999999" hint="Informe DDI + DDD + número ou um JID completo." />}
        {fields}
        {!operationId.startsWith("send-status") && !["send-button", "send-list", "send-carousel"].includes(operationId) && <CommonFields data={data} patch={patch} />}
      </div>
      <Preview operationId={operationId} data={data} />
    </div>
  );
}

function requiredString(data: JsonObject, key: string, label: string, errors: string[]) {
  if (!stringValue(data[key]).trim()) errors.push(`${label} é obrigatório.`);
}

export function validateRequestDraft(operationId: string, bodyMode: BodyMode, body: string, file: File | null): RequestValidation {
  const errors: string[] = [];
  const warnings: string[] = [];
  if (bodyMode === "none") return { errors, warnings };

  const parsed = parseObject(body);
  if (!parsed.value) return { errors: [`JSON inválido: ${parsed.error}`], warnings };
  const data = parsed.value;

  if (supportsGuidedRequest(operationId) && !operationId.startsWith("send-status")) {
    requiredString(data, "number", "Destinatário", errors);
  }
  if (bodyMode === "multipart" && ["send-media-file", "send-status-media-file"].includes(operationId) && !file) {
    errors.push("Selecione o arquivo do upload multipart.");
  }

  switch (operationId) {
    case "send-text": requiredString(data, "text", "Texto", errors); break;
    case "send-link": requiredString(data, "text", "Texto", errors); break;
    case "send-media-url":
    case "send-status-media-url": requiredString(data, "url", "URL/base64", errors); break;
    case "send-poll":
      requiredString(data, "question", "Pergunta", errors);
      if (arrayValue(data.options).filter((item) => stringValue(item).trim()).length < 2) errors.push("A enquete precisa de pelo menos duas opções.");
      break;
    case "send-sticker": requiredString(data, "sticker", "Figurinha", errors); break;
    case "send-location":
      requiredString(data, "name", "Nome do local", errors);
      requiredString(data, "address", "Endereço", errors);
      if (!numberValue(data.latitude)) errors.push("Latitude diferente de zero é obrigatória.");
      if (!numberValue(data.longitude)) errors.push("Longitude diferente de zero é obrigatória.");
      break;
    case "send-contact": {
      const vcard = objectValue(data.vcard);
      requiredString(vcard, "fullName", "Nome do contato", errors);
      requiredString(vcard, "phone", "Telefone do contato", errors);
      break;
    }
    case "send-button": {
      requiredString(data, "title", "Título", errors);
      requiredString(data, "description", "Descrição", errors);
      requiredString(data, "footer", "Rodapé", errors);
      const buttons = arrayValue(data.buttons).map(objectValue);
      if (!buttons.length) errors.push("Adicione pelo menos um botão.");
      const types = buttons.map((item) => stringValue(item.type).toLowerCase());
      if (types.filter((item) => item === "reply").length > 3) errors.push("São permitidos no máximo três botões reply.");
      if (types.includes("reply") && types.some((item) => item !== "reply")) errors.push("Botões reply não podem ser misturados com CTA.");
      if (types.includes("pix") && buttons.length !== 1) errors.push("PIX precisa ser o único botão da mensagem.");
      break;
    }
    case "send-list": {
      requiredString(data, "title", "Título", errors);
      requiredString(data, "description", "Descrição", errors);
      requiredString(data, "buttonText", "Texto do botão", errors);
      const sections = arrayValue(data.sections).map(objectValue);
      if (!sections.some((section) => arrayValue(section.rows).length > 0)) errors.push("A lista precisa de pelo menos uma linha.");
      break;
    }
    case "send-carousel":
      if (!arrayValue(data.cards).length) errors.push("O carrossel precisa de pelo menos um card.");
      break;
    case "send-status-text": requiredString(data, "text", "Texto do status", errors); break;
  }

  if (stringValue(data.number).includes("9999999999")) warnings.push("O destinatário ainda parece ser o número de exemplo.");
  if (body.includes("CHAVE_PIX") || body.includes("ID_DA_MENSAGEM")) warnings.push("O payload contém valores de exemplo que precisam ser substituídos.");
  return { errors, warnings };
}
