// #967：TS 7（原生重写版）无 TS JS API——治理扫描器不再依赖 typescript
// 包做 AST 解析。本模块在原始源码上产出「掩码区间」：注释整体、字符串
// /模板字面量的字面 chunk（`${...}` 插值是真代码不掩码）、正则字面量
// 内容。调用方在原始源码上跑正则，命中起点落在任一区间内即视为注释/
// 文案假匹配予以丢弃——比整段空白化精确：`window['confirm'](` 与
// `` `${t('key')}` `` 这类「字符串/模板内嵌代码」形态不被误杀。
//
// 与 TS AST 的语义差异（同 strict-scan 姿势，均为已记录的刻意限制）：
// - 撇号启发：紧跟标识符的 `'`（JSX 文案 it's）不视为字符串开符；
//   `throw'x'` 这类紧贴写法（合法但不存在于本仓）会被漏掩。
// - 正则/division 歧义用「表达式位置」启发（前一实义 token 是运算符/
//   括号/关键字则开正则，是标识符/`)`/`]`/字面量则当除法）；JSX 闭合
//   标签 `</` 特判不开正则。启发失手的形态（如 `case "x": /re/`）会
//   误开正则态——当前产出源集实测无此形态（#967 全量等价对照）。
// - 未闭合的引号/花括号会掩到文件尾（TS 解析器同样报错，不会静默）。

const REGEX_POSITION_KEYWORDS = new Set([
  "return", "typeof", "instanceof", "in", "of", "new", "delete", "void",
  "throw", "case", "do", "else", "yield", "await",
]);

/**
 * 返回 [start, end) 掩码区间数组（文档序）。
 */
export function maskedRegions(source) {
  const spans = [];
  const push = (start, end) => {
    if (end > start) spans.push([start, end]);
  };
  // 上下文栈：code(depth 计花括号)，string(quote)，template，regex，
  // 注释。模板 `${` 入 code 态、配对 `}` 回模板态——插值表达式按代码处理。
  const stack = [{ kind: "code", depth: 0 }];
  let chunkStart = -1; // 当前模板 chunk 起点（chunk = 两个插值之间的字面文本）
  const top = () => stack[stack.length - 1];
  // code 态的正则/division 判别：lastWord 在读标识符，prevSig 是上一个
  // 实义 token 的类别（"e" 表达式位置可开正则，"v" 值位置当除法）。
  let lastWord = "";
  let prevSig = "e";
  const flushWord = () => {
    if (lastWord) {
      prevSig = REGEX_POSITION_KEYWORDS.has(lastWord) ? "e" : "v";
      lastWord = "";
    }
  };
  let i = 0;
  while (i < source.length) {
    const ctx = top();
    const ch = source[i];
    const next = i + 1 < source.length ? source[i + 1] : "";
    if (ctx.kind === "lineComment") {
      if (ch === "\n" || i === source.length - 1) {
        push(ctx.start, ch === "\n" ? i : i + 1);
        stack.pop();
      }
      i++;
      continue;
    }
    if (ctx.kind === "blockComment") {
      if (ch === "*" && next === "/") {
        push(ctx.start, i + 2);
        stack.pop();
        i += 2;
        continue;
      }
      i++;
      continue;
    }
    if (ctx.kind === "string") {
      if (ch === "\\") {
        i += 2;
        continue;
      }
      if (ch === ctx.quote || ch === "\n") {
        // 内容掩码（定界符不掩，保持形态可辨）；换行仍未闭合按行内近似收尾。
        push(ctx.start + 1, i);
        stack.pop();
        prevSig = "v";
      }
      i++;
      continue;
    }
    if (ctx.kind === "template") {
      if (ch === "\\") {
        i += 2;
        continue;
      }
      if (ch === "`") {
        push(chunkStart, i);
        stack.pop();
        prevSig = "v";
        i++;
        continue;
      }
      if (ch === "$" && next === "{") {
        push(chunkStart, i);
        stack.push({ kind: "code", depth: 0 });
        i += 2;
        continue;
      }
      i++;
      continue;
    }
    if (ctx.kind === "regex") {
      if (ch === "\\") {
        i += 2;
        continue;
      }
      if (ch === "\n") {
        // 正则不跨行：按行内近似收尾（源码出现即本身非法）。
        push(ctx.start + 1, i);
        stack.pop();
        prevSig = "v";
        i++;
        continue;
      }
      if (ch === "[") ctx.inClass = true;
      else if (ch === "]") ctx.inClass = false;
      else if (ch === "/" && !ctx.inClass) {
        push(ctx.start + 1, i);
        stack.pop();
        prevSig = "v";
      }
      i++;
      continue;
    }
    // code 上下文
    if (ch === "/" && next === "/") {
      flushWord();
      stack.push({ kind: "lineComment", start: i });
      i += 2;
      continue;
    }
    if (ch === "/" && next === "*") {
      flushWord();
      stack.push({ kind: "blockComment", start: i });
      i += 2;
      continue;
    }
    if (ch === "'" || ch === '"') {
      const prev = i > 0 ? source[i - 1] : "";
      if (/[A-Za-z0-9_$]/.test(prev)) {
        // it's 一类撇号（紧跟标识符）：不是字符串开符。
        i++;
        continue;
      }
      flushWord();
      stack.push({ kind: "string", quote: ch, start: i });
      prevSig = "v";
      i++;
      continue;
    }
    if (ch === "`") {
      flushWord();
      stack.push({ kind: "template" });
      chunkStart = i + 1;
      prevSig = "v";
      i++;
      continue;
    }
    if (ch === "/") {
      // 表达式位置 → 正则字面量；值位置 → 除法。紧贴 `<`（JSX 闭合标签
      // `</p>`）不开正则。
      const afterLT = i > 0 && source[i - 1] === "<";
      if (!afterLT && prevSig === "e") {
        flushWord();
        stack.push({ kind: "regex", start: i, inClass: false });
        prevSig = "v";
        i++;
        continue;
      }
      flushWord();
      prevSig = "v";
      i++;
      continue;
    }
    if (/[A-Za-z0-9_$]/.test(ch)) {
      lastWord += ch;
      i++;
      continue;
    }
    flushWord();
    if (/\s/.test(ch)) {
      i++;
      continue;
    }
    if (ch === "{") {
      ctx.depth++;
      prevSig = "e";
      i++;
      continue;
    }
    if (ch === "}") {
      if (ctx.depth === 0 && stack.length > 1 && stack[stack.length - 2].kind === "template") {
        chunkStart = i + 1;
        stack.pop();
        prevSig = "v";
        i++;
        continue;
      }
      if (ctx.depth > 0) ctx.depth--;
      // 值位置：JSX 表达式闭合后的 `/>` 与文本斜杠（} / {）不当正则。
      prevSig = "v";
      i++;
      continue;
    }
    // 其余符号：运算符/括号按表达式位置，`)`/`]` 按值位置。
    prevSig = ch === ")" || ch === "]" ? "v" : "e";
    i++;
  }
  return spans;
}

/** 源码偏移 → 1-based 行号。 */
export function lineAt(source, index) {
  let line = 1;
  for (let i = 0; i < index && i < source.length; i++) {
    if (source[i] === "\n") line++;
  }
  return line;
}
