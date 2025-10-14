/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable @typescript-eslint/no-explicit-any */
// Utility helper functions for Astra Chat UI

export function isJsonString(str: string): boolean {
  if (typeof str !== "string") return false;
  try {
    const parsed = JSON.parse(str);
    return typeof parsed === "object" && parsed !== null;
  } catch {
    return false;
  }
}

export function parseMaybeJson(input: any): any {
  if (typeof input !== "string") return input;
  try {
    const parsed = JSON.parse(input);
    if (typeof parsed === "object" && parsed !== null) {
      for (const key in parsed) {
        if (typeof parsed[key] === "string" && isJsonString(parsed[key])) {
          parsed[key] = parseMaybeJson(parsed[key]);
        }
      }
    }
    return parsed;
  } catch {
    return input;
  }
}

// Cleans up raw chat message content, unescaping and trimming as needed
export function cleanContent(raw: string | undefined): string {
  if (!raw) return "";
  let content = raw.trim();
  try {
    if (
      (content.startsWith('"') && content.endsWith('"')) ||
      (content.startsWith("'") && content.endsWith("'"))
    ) {
      if (content.startsWith("'") && content.endsWith("'")) {
        content = '"' + content.slice(1, -1).replace(/"/g, '\\"') + '"';
      }
      content = JSON.parse(content);
    }
  } catch (e) {
    //
  }
  content = content
    .replace(/\\n/g, "\n")
    .replace(/\\r/g, "\r")
    .replace(/\\t/g, "\t")
    .replace(/\\"/g, '"')
    .replace(/\\'/g, "'")
    .replace(/\\\\/g, "\\");
  return content.trim();
}

export function getCurrentTime(): string {
  const now = new Date();
  return now.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

// Scrolls a ref to bottom smoothly if supported
export function scrollToBottom(ref: React.RefObject<HTMLDivElement | null>) {
  if (ref.current) {
    ref.current.scrollIntoView({ behavior: "smooth" });
  }
}

