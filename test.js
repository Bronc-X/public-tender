const fs = require('fs');
const original_markdown = fs.readFileSync('data/files/incoming/02a47a5f-8bf7-4260-9ab7-a00caf9ee0fb_template_structured.md', 'utf-8');
const activeChapter = "一、投标函";

const cleanActiveChapter = activeChapter.replace(/^第[一二三四五六七八九十百]+章/g, '').trim();
const lines = original_markdown.split('\n');
let capturing = false;
let captureLevel = 0;
const result = [];

for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const headingMatch = line.match(/^(#{1,6})\s+(.*)/);
    
    if (headingMatch) {
        const level = headingMatch[1].length;
        const title = headingMatch[2].trim();
        
        if (!capturing) {
            if (title.includes(activeChapter) || (cleanActiveChapter && title.includes(cleanActiveChapter))) {
                capturing = true;
                captureLevel = level;
                result.push(line);
            }
        } else {
            if (level <= captureLevel) {
                break;
            } else {
                result.push(line);
            }
        }
    } else if (capturing) {
        result.push(line);
    }
}
const ans = result.length > 0 ? result.join('\n') : original_markdown;
console.log("Result length:", result.length);
console.log("Ans length:", ans.length);
