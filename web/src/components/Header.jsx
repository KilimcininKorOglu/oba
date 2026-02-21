export default function Header({ title, actions }) {
  return (
    <header className="flex items-center justify-between mb-6">
      <h1 className="text-2xl font-semibold text-zinc-100">{title}</h1>
      {actions && <div className="flex items-center gap-3">{actions}</div>}
    </header>
  );
}
