// formatFileLabel builds the "Title - filename" label shared by the browser
// tab title and the content-area title bar. Falls back to the file name alone
// when the file has no usable title (undefined, empty, or whitespace-only, e.g.
// no heading or a non-markdown file).
export function formatFileLabel(name: string, title?: string): string {
  return title && title.trim() !== "" ? `${title} - ${name}` : name;
}
