import "@testing-library/jest-dom/vitest";

// jsdom implements <canvas> only as a stub: getContext throws "Not
// implemented". @xterm/xterm (mounted by the lab terminal) calls
// getContext("2d") on render, so any test that renders a tree containing the
// terminal - App.test.tsx mounts <App/>, whose routes pull it in - crashes
// before a single assertion runs. A minimal 2D-context stub lets those renders
// succeed; the terminal isn't what these tests exercise, so fidelity doesn't
// matter, only that it doesn't throw.
const noop = () => {};
const stubContext = {
  fillRect: noop,
  clearRect: noop,
  getImageData: (_x: number, _y: number, w: number, h: number) => ({ data: new Array(w * h * 4) }),
  putImageData: noop,
  createImageData: () => [],
  setTransform: noop,
  drawImage: noop,
  save: noop,
  restore: noop,
  beginPath: noop,
  moveTo: noop,
  lineTo: noop,
  closePath: noop,
  stroke: noop,
  translate: noop,
  scale: noop,
  rotate: noop,
  arc: noop,
  fill: noop,
  measureText: () => ({ width: 0 }),
  transform: noop,
  rect: noop,
  clip: noop,
  fillText: noop,
  strokeText: noop,
  createLinearGradient: () => ({ addColorStop: noop }),
};
HTMLCanvasElement.prototype.getContext = (() => stubContext) as unknown as typeof HTMLCanvasElement.prototype.getContext;
