import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { HoldCrossfadeImage } from "@/components/content/HoldCrossfadeImage";
import { act, cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";

test.afterEach(() => cleanup());
installDom();

/* #430 首帧渐进双层契约：入场窗保持层唯一可见（零换图）；hold 释放且规范层
   就绪后保持层 180ms 淡出并卸载；规范层未就绪/失败时保持层永不下岗。 */
const HOLD = "/_next/image?url=%2Fcovers%2Fcard.jpg&w=420&q=75";
const CANONICAL = "/_next/image?url=%2Fcovers%2Fcard.jpg&w=1080&q=75";

function holdLayer(view: { container: HTMLElement }) {
  return view.container.querySelector<HTMLImageElement>('img[data-slot="cover-hold"]');
}
function canonicalLayer(view: { container: HTMLElement }) {
  return Array.from(view.container.querySelectorAll("img")).find(
    (img) => img.getAttribute("data-slot") !== "cover-hold",
  );
}

test("#430 hold window: hold layer is the only visible layer and drives settle", async () => {
  const settled: string[] = [];
  const view = render(
    <HoldCrossfadeImage
      canonicalSrc={CANONICAL}
      holdSrc={HOLD}
      alt="cover"
      imgClassName="object-cover"
      onSettle={(state) => settled.push(state)}
    />,
  );
  const imgs = Array.from(view.container.querySelectorAll("img"));
  assert.equal(imgs.length, 2, "hold window renders exactly the two layers");
  assert.equal(canonicalLayer(view)?.getAttribute("src"), CANONICAL);
  assert.equal(holdLayer(view)?.getAttribute("src"), HOLD);
  assert.equal(canonicalLayer(view)?.style.opacity, "0", "canonical invisible during the motion window");
  assert.equal(holdLayer(view)?.style.opacity, "1", "hold layer fully visible during the motion window");

  await act(async () => {
    fireEvent.load(holdLayer(view)!);
    await Promise.resolve();
  });
  assert.deepEqual(settled, ["ready"], "cached quick variant settles the first frame instantly");

  /* 规范层就绪但 hold 未释放：仍不淡出（动效期间零换图）。 */
  await act(async () => {
    fireEvent.load(canonicalLayer(view)!);
    await Promise.resolve();
  });
  assert.equal(holdLayer(view)?.style.opacity, "1", "ready-but-unreleased canonical must not trigger the fade");
});

test("#430 release: fade starts only after hold released AND canonical ready, then hold unmounts", async () => {
  const released: string[] = [];
  function Frame({ holdSrc }: { holdSrc: string | null }) {
    return (
      <HoldCrossfadeImage
        canonicalSrc={CANONICAL}
        holdSrc={holdSrc}
        alt="cover"
        onHoldReleased={() => released.push("released")}
      />
    );
  }
  const view = render(<Frame holdSrc={HOLD} />);

  /* 释放但规范层未就绪：保持层继续顶着（不闪空白）。 */
  await act(async () => {
    view.rerender(<Frame holdSrc={null} />);
    await Promise.resolve();
  });
  assert.ok(holdLayer(view), "hold layer survives the release until the canonical layer is ready");
  assert.equal(holdLayer(view)?.style.opacity, "1", "not-ready canonical keeps the hold layer opaque");

  await act(async () => {
    fireEvent.load(canonicalLayer(view)!);
    await Promise.resolve();
  });
  assert.equal(holdLayer(view)?.style.opacity, "0", "release + decode-ready starts the 180ms fade-out");

  await act(async () => {
    fireEvent.transitionEnd(holdLayer(view)!, { propertyName: "opacity" });
    await Promise.resolve();
  });
  assert.ok(!holdLayer(view), "hold layer unmounts after the fade completes");
  assert.deepEqual(released, ["released"], "release hook fired once for mechanism verification");
});

test("#430 canonical failure while holding keeps the hold layer forever (no blank flash)", async () => {
  function Frame({ holdSrc }: { holdSrc: string | null }) {
    return <HoldCrossfadeImage canonicalSrc={CANONICAL} holdSrc={holdSrc} alt="cover" />;
  }
  const view = render(<Frame holdSrc={HOLD} />);
  await act(async () => {
    fireEvent.error(canonicalLayer(view)!);
    await Promise.resolve();
  });
  await act(async () => {
    view.rerender(<Frame holdSrc={null} />);
    await Promise.resolve();
  });
  const hold = holdLayer(view);
  assert.ok(hold, "failed canonical keeps the hold layer mounted");
  assert.equal(hold?.style.opacity, "1", "failed canonical never triggers the fade");
});

test("#430 no hold anchor: single canonical layer, settle driven by canonical load", async () => {
  const settled: string[] = [];
  const view = render(
    <HoldCrossfadeImage canonicalSrc={CANONICAL} alt="cover" onSettle={(s) => settled.push(s)} />,
  );
  const imgs = view.container.querySelectorAll("img");
  assert.equal(imgs.length, 1, "no anchor renders the canonical layer only");
  assert.equal(imgs[0].getAttribute("src"), CANONICAL);
  assert.equal(imgs[0].style.opacity, "1");
  await act(async () => {
    fireEvent.load(imgs[0]);
    await Promise.resolve();
  });
  assert.deepEqual(settled, ["ready"]);
});

test("#430 settle fires once across both layers (cached hold + canonical)", async () => {
  const settled: string[] = [];
  const view = render(
    <HoldCrossfadeImage canonicalSrc={CANONICAL} holdSrc={HOLD} alt="cover" onSettle={(s) => settled.push(s)} />,
  );
  await act(async () => {
    fireEvent.load(holdLayer(view)!);
    fireEvent.load(canonicalLayer(view)!);
    await Promise.resolve();
  });
  assert.deepEqual(settled, ["ready"], "canonical load never re-settles an already-settled frame");
});
