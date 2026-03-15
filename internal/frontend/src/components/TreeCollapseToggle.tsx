import { CollapseIcon } from "./icons/CollapseIcon";
import { ExpandIcon } from "./icons/ExpandIcon";

interface TreeCollapseToggleProps {
  collapsed: boolean;
  onToggle: () => void;
}

export function TreeCollapseToggle({ collapsed, onToggle }: TreeCollapseToggleProps) {
  return (
    <button
      type="button"
      className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-header-text cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
      onClick={onToggle}
      title={collapsed ? "Expand all" : "Collapse all"}
      aria-label={collapsed ? "Expand all" : "Collapse all"}
    >
      {collapsed ? <ExpandIcon /> : <CollapseIcon />}
    </button>
  );
}
