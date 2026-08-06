export interface MasonryPosition {
  top: number;
  left: number;
  width: number;
}

export interface MasonryLayout {
  positions: MasonryPosition[];
  height: number;
}

/**
 * 最短列瀑布流布局：按 items 顺序逐条放入当前累计高度最短列（列宽等分、
 * 固定列间距）。DOM/键盘/读屏顺序保持 items 原顺序（返回的 positions 与
 * 输入下标一一对应），配合绝对定位渲染即得到真正的按高度平衡的瀑布流。
 */
export function computeShortestColumnLayout(
  heights: number[],
  columnCount: number,
  gap: number,
  columnWidth: number,
): MasonryLayout {
  const columns = Math.max(1, Math.floor(columnCount));
  const columnHeights = new Array<number>(columns).fill(0);
  const positions: MasonryPosition[] = new Array(heights.length);

  for (let index = 0; index < heights.length; index += 1) {
    let column = 0;
    for (let candidate = 1; candidate < columns; candidate += 1) {
      if (columnHeights[candidate] < columnHeights[column]) column = candidate;
    }
    const top = columnHeights[column];
    columnHeights[column] = top + Math.max(0, heights[index]) + gap;
    positions[index] = { top, left: column * (columnWidth + gap), width: columnWidth };
  }

  const maxColumn = columns > 0 ? Math.max(...columnHeights) : 0;
  return { positions, height: Math.max(0, maxColumn - gap) };
}

export function masonryColumnCount(viewportWidth: number): number {
  if (viewportWidth <= 700) return 2;
  if (viewportWidth <= 1100) return 3;
  return 4;
}

export const MASONRY_GAP = 16; // gap-4
