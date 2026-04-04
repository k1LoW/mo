interface CloseFileButtonProps {
  onClose: () => void;
  uploaded?: boolean;
}

export function CloseFileButton({ onClose, uploaded }: CloseFileButtonProps) {
  return (
    <button
      className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
      onClick={onClose}
      title={uploaded ? "Discard" : "Close file"}
    >
      <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        {uploaded ? (
          <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
        ) : (
          <>
            <path strokeLinecap="round" strokeLinejoin="round" d="M14.25 2.25H6a1.5 1.5 0 0 0-1.5 1.5v16.5a1.5 1.5 0 0 0 1.5 1.5h3M14.25 2.25V7.5H19.5V10M14.25 2.25L19.5 7.5" />
            <path strokeLinecap="round" strokeLinejoin="round" d="M13 13.5l8 8m0-8-8 8" />
          </>
        )}
      </svg>
    </button>
  );
}
