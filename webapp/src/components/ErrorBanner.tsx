/**
 * ErrorBanner renders a styled error message box with a warning icon.
 * Returns null when message is empty so callers can render it unconditionally.
 */
export default function ErrorBanner({ message }: { message: string }) {
  if (!message) return null;

  return (
    <div
      role="alert"
      className="flex items-start gap-2 rounded-md border-l-4 border-red-500 bg-red-50 px-3 py-2 text-sm text-red-700"
    >
      <WarningIcon />
      <span>{message}</span>
    </div>
  );
}

function WarningIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 20 20"
      fill="currentColor"
      className="mt-0.5 h-4 w-4 shrink-0"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z"
        clipRule="evenodd"
      />
    </svg>
  );
}
