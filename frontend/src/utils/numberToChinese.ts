/**
 * 将阿拉伯数字转换为中文数字 (用于章节编号)
 */
export const numberToChinese = (num: number): string => {
    const chineseNums = ['零', '一', '二', '三', '四', '五', '六', '七', '八', '九'];
    const units = ['', '十', '百', '千', '万'];
    
    if (num === 0) return chineseNums[0];
    if (num === 10) return '十';
    if (num > 10 && num < 20) return '十' + chineseNums[num % 10];
    
    let res = '';
    let str = num.toString();
    for (let i = 0; i < str.length; i++) {
        let n = parseInt(str[i]);
        let unit = units[str.length - i - 1];
        if (n !== 0) {
            res += chineseNums[n] + unit;
        } else {
            if (res.length > 0 && res[res.length - 1] !== chineseNums[0]) {
                res += chineseNums[0];
            }
        }
    }
    if (res.endsWith('零')) res = res.substring(0, res.length - 1);
    return res;
};

/**
 * 根据层级获取标签 (1 -> 第一章, 1 -> 第一节, 1 -> (一))
 */
export const getChapterLabel = (order: number, level: 'chapter' | 'unit' | 'subsection' | number): string => {
    const chinese = numberToChinese(order);
    if (level === 'chapter' || level === 1) return `第${chinese}章`;
    if (level === 'unit' || level === 2) return `第${chinese}节`;
    if (level === 'subsection' || level === 3) return `(${chinese})`;
    return chinese;
};
