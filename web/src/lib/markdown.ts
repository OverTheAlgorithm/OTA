/**
 * Simple markdown-to-HTML renderer for static legal/info pages.
 * Handles headings, bold, hr, tables, numbered lists, links, and paragraphs.
 */
export function renderMarkdown(md: string): string {
  const lines = md.split("\n");
  const output: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Headings
    if (line.startsWith("# ")) {
      output.push(`<h1>${inlineRender(line.slice(2))}</h1>`);
      i++;
      continue;
    }
    if (line.startsWith("## ")) {
      output.push(`<h2>${inlineRender(line.slice(3))}</h2>`);
      i++;
      continue;
    }
    if (line.startsWith("### ")) {
      output.push(`<h3>${inlineRender(line.slice(4))}</h3>`);
      i++;
      continue;
    }
    if (line.startsWith("#### ")) {
      output.push(`<h4>${inlineRender(line.slice(5))}</h4>`);
      i++;
      continue;
    }

    // Horizontal rule
    if (line.trim() === "---") {
      output.push("<hr>");
      i++;
      continue;
    }

    // Table: collect consecutive table rows
    if (line.startsWith("|")) {
      const tableLines: string[] = [];
      while (i < lines.length && lines[i].startsWith("|")) {
        tableLines.push(lines[i]);
        i++;
      }
      output.push(renderTable(tableLines));
      continue;
    }

    // Numbered list: collect consecutive numbered list items
    if (/^\d+\.\s/.test(line)) {
      const listItems: string[] = [];
      while (i < lines.length && /^\d+\.\s/.test(lines[i])) {
        listItems.push(lines[i].replace(/^\d+\.\s/, ""));
        i++;
      }
      const items = listItems.map((item) => `<li>${inlineRender(item)}</li>`).join("");
      output.push(`<ol>${items}</ol>`);
      continue;
    }

    // Blank line
    if (line.trim() === "") {
      i++;
      continue;
    }

    // Paragraph: collect consecutive non-blank, non-special lines
    const paragraphLines: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !lines[i].startsWith("#") &&
      !lines[i].startsWith("|") &&
      lines[i].trim() !== "---" &&
      !/^\d+\.\s/.test(lines[i])
    ) {
      paragraphLines.push(lines[i]);
      i++;
    }
    if (paragraphLines.length > 0) {
      output.push(`<p>${inlineRender(paragraphLines.join(" "))}</p>`);
    }
  }

  return output.join("\n");
}

function renderTable(tableLines: string[]): string {
  // Filter out separator rows (e.g. |---|---|)
  const dataRows = tableLines.filter((l) => !/^\|[-\s|:]+\|$/.test(l.trim()));
  if (dataRows.length === 0) return "";

  const [headerRow, ...bodyRows] = dataRows;
  const headerCells = parseCells(headerRow);
  const thead = `<thead><tr>${headerCells.map((c) => `<th>${inlineRender(c)}</th>`).join("")}</tr></thead>`;

  const tbody =
    bodyRows.length > 0
      ? `<tbody>${bodyRows
          .map((row) => {
            const cells = parseCells(row);
            return `<tr>${cells.map((c) => `<td>${inlineRender(c)}</td>`).join("")}</tr>`;
          })
          .join("")}</tbody>`
      : "";

  return `<table>${thead}${tbody}</table>`;
}

function parseCells(row: string): string[] {
  return row
    .split("|")
    .slice(1, -1)
    .map((c) => c.trim());
}

function inlineRender(text: string): string {
  // Links [text](url)
  text = text.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');
  // Bold **text**
  text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  return text;
}
