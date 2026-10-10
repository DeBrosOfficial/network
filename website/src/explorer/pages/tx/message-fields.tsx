import type { MessageField, TxMessage } from "../../model/types";
import { Amount } from "../../ui/amount";

function FieldValue({ field }: { field: MessageField }) {
  if (field.kind === "norama") return <Amount norama={field.value} />;
  return <span className={field.kind === "address" ? "font-mono text-xs" : undefined}>{field.value}</span>;
}

/**
 * The structured facts of an action message, under its sentence. Every value
 * is chain-written text that the decoder already cleaned and bounded; it is
 * rendered as text, never as markup.
 */
export function MessageFields({ message }: { message: TxMessage }) {
  if (message.type !== "action" || message.fields.length === 0) return null;
  return (
    <dl className="mt-1 grid grid-cols-[minmax(0,9rem)_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
      {message.fields.map((field) => (
        <div key={field.label} className="contents">
          <dt className="text-muted">{field.label}</dt>
          <dd className="m-0 [overflow-wrap:anywhere]">
            <FieldValue field={field} />
          </dd>
        </div>
      ))}
    </dl>
  );
}
