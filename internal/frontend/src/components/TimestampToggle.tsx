import { CalendarIcon } from "./icons/CalendarIcon";
import { ClockIcon } from "./icons/ClockIcon";

export type TimestampMode = "off" | "relative" | "absolute";

interface TimestampToggleProps {
  mode: TimestampMode;
  onToggle: () => void;
}

const titles: Record<TimestampMode, string> = {
  off: "Show relative timestamps",
  relative: "Show absolute timestamps",
  absolute: "Hide timestamps",
};

export function TimestampToggle({ mode, onToggle }: TimestampToggleProps) {
  return (
    <button
      type="button"
      className={`flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover ${
        mode !== "off" ? "text-gh-header-text bg-gh-bg-hover" : "text-gh-header-text opacity-50"
      }`}
      onClick={onToggle}
      aria-label={titles[mode]}
      title={titles[mode]}
    >
      {mode === "absolute" ? <CalendarIcon /> : <ClockIcon />}
    </button>
  );
}
