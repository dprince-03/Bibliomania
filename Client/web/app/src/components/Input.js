export default function Input({ label, id, ...rest }) {
  return (
    <div>
      <label htmlFor={id} className="block text-sm font-medium text-foreground">
        {label}
      </label>
      <input
        id={id}
        name={id}
        className="mt-1.5 w-full rounded-lg border border-border bg-background px-3.5 py-2.5 text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
        {...rest}
      />
    </div>
  );
}
