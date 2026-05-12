import re

with open('data/files/incoming/02a47a5f-8bf7-4260-9ab7-a00caf9ee0fb_template_structured.md', 'r') as f:
    text = f.read()

activeChapter = "一、投标函"
cleanActiveChapter = re.sub(r'^第[一二三四五六七八九十百]+章', '', activeChapter).strip()

lines = text.split('\n')
capturing = False
captureLevel = 0
result = []

for line in lines:
    headingMatch = re.match(r'^(#{1,6})\s+(.*)', line)
    
    if headingMatch:
        level = len(headingMatch.group(1))
        title = headingMatch.group(2).strip()
        
        if not capturing:
            if activeChapter in title or (cleanActiveChapter and cleanActiveChapter in title):
                capturing = True
                captureLevel = level
                result.append(line)
        else:
            if level <= captureLevel:
                break
            else:
                result.append(line)
    elif capturing:
        result.append(line)

print("length:", len(result))
print("\n".join(result[:10]))
