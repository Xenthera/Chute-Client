export class ChuteCanvas {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private doorTarget = 0;
  private doorProgress = 0;
  private rafId: number | null = null;
  private readonly lerpFactor = 0.08;
  private readonly tunnelSpeed = 0.44;
  private readonly tunnelRadius = 18;
  private readonly handleResize: () => void;
  private pulseDirection: "in" | "out" = "in";
  private pulseColors = {
    base: { r: 130, g: 130, b: 130 },
    glow: { r: 80, g: 255, b: 255 },
  };

  constructor(canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      throw new Error("Failed to acquire canvas context.");
    }
    this.canvas = canvas;
    this.ctx = ctx;
    this.handleResize = this.resize.bind(this);
    this.resize();
    window.addEventListener("resize", this.handleResize);
  }

  start() {
    if (this.rafId !== null) {
      return;
    }
    const tick = (now: number) => {
      this.render(now);
      this.rafId = requestAnimationFrame(tick);
    };
    this.rafId = requestAnimationFrame(tick);
  }

  setConnected(connected: boolean) {
    this.doorTarget = connected ? 1 : 0;
  }

  setRole(role: "sender" | "receiver" | null) {
    if (role === "receiver") {
      this.pulseDirection = "out";
      this.pulseColors = {
        base: { r: 130, g: 130, b: 130 },
        glow: { r: 255, g: 150, b: 60 },
      };
      return;
    }
    this.pulseDirection = "in";
    this.pulseColors = {
      base: { r: 130, g: 130, b: 130 },
      glow: { r: 80, g: 255, b: 255 },
    };
  }

  destroy() {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
    window.removeEventListener("resize", this.handleResize);
  }

  private render(now: number) {
    const rect = this.canvas.getBoundingClientRect();
    this.doorProgress += (this.doorTarget - this.doorProgress) * this.lerpFactor;
    if (Math.abs(this.doorTarget - this.doorProgress) < 0.001) {
      this.doorProgress = this.doorTarget;
    }
    const tunnelPhase = ((now / 1000) * this.tunnelSpeed) % 1;
    this.drawFunnelDoors(rect.width, rect.height, this.doorProgress, tunnelPhase);
  }

  private resize() {
    const rect = this.canvas.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    this.canvas.width = Math.max(1, Math.floor(rect.width * dpr));
    this.canvas.height = Math.max(1, Math.floor(rect.height * dpr));
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    this.ctx.clearRect(0, 0, rect.width, rect.height);
  }

  private getSurfaceColor() {
    const root = getComputedStyle(document.documentElement);
    const value = root.getPropertyValue("--surface").trim();
    return value || "#e7edf4";
  }

  private drawRoundedRect(
    x: number,
    y: number,
    width: number,
    height: number,
    radius: number
  ) {
    const r = Math.max(0, Math.min(radius, Math.min(width, height) / 2));
    this.ctx.beginPath();
    this.ctx.moveTo(x + r, y);
    this.ctx.lineTo(x + width - r, y);
    this.ctx.quadraticCurveTo(x + width, y, x + width, y + r);
    this.ctx.lineTo(x + width, y + height - r);
    this.ctx.quadraticCurveTo(x + width, y + height, x + width - r, y + height);
    this.ctx.lineTo(x + r, y + height);
    this.ctx.quadraticCurveTo(x, y + height, x, y + height - r);
    this.ctx.lineTo(x, y + r);
    this.ctx.quadraticCurveTo(x, y, x + r, y);
    this.ctx.closePath();
  }

  private drawTunnel(width: number, height: number, phase: number) {
    this.ctx.fillStyle = "#0a0f16";
    this.ctx.fillRect(0, 0, width, height);

    const rings = 20;
    const minSize = Math.min(width, height);
    const baseRings = 10;
    const baseGap = Math.max(6, minSize / (baseRings * 1.4));
    const desiredMaxInset = (baseRings - 1) * baseGap + 2;
    const gap = Math.max(4, (desiredMaxInset - 2) / (rings - 1));
    const baseColor = this.pulseColors.base;
    const glowColor = this.pulseColors.glow;

    for (let i = 0; i < rings; i += 1) {
      const inset = i * gap + 2;
      const ringWidth = Math.max(0, width - inset * 2);
      const ringHeight = Math.max(0, height - inset * 2);
      if (ringWidth <= 0 || ringHeight <= 0) {
        continue;
      }
      const localPhase =
        this.pulseDirection === "in" ? (1 - phase + i * 0.12) % 1 : (phase + i * 0.12) % 1;
      const glow = Math.max(0, Math.sin(localPhase * Math.PI * 2));
      const fadeToCenter = Math.max(0, 1 - (i / (rings - 1)) * 1.5);
      if (fadeToCenter <= 0) {
        continue;
      }
      const r = Math.round(baseColor.r + (glowColor.r - baseColor.r) * glow);
      const g = Math.round(baseColor.g + (glowColor.g - baseColor.g) * glow);
      const b = Math.round(baseColor.b + (glowColor.b - baseColor.b) * glow);
      this.ctx.strokeStyle = `rgba(${r}, ${g}, ${b}, ${fadeToCenter})`;
      this.ctx.lineWidth = 2;
      this.drawRoundedRect(inset, inset, ringWidth, ringHeight, this.tunnelRadius);
      this.ctx.stroke();
    }
  }

  private drawFunnelDoors(width: number, height: number, doorProgress: number, tunnelPhase: number) {
    this.ctx.clearRect(0, 0, width, height);
    this.drawTunnel(width, height, tunnelPhase);

    const doorColor = this.getSurfaceColor();
    const borderColor = "#b6bcc4";
    const half = width / 2;
    const offset = half * doorProgress;
    const leftX = -offset;
    const rightX = half + offset;
    const doorY = -2;
    const doorHeight = height + 4;

    this.ctx.fillStyle = doorColor;
    this.ctx.fillRect(leftX, doorY, half, doorHeight);
    this.ctx.fillRect(rightX, doorY, half, doorHeight);

    this.ctx.strokeStyle = borderColor;
    this.ctx.lineWidth = 1;
    this.ctx.strokeRect(leftX + 0.5, doorY + 0.5, half - 1, doorHeight - 1);
    this.ctx.strokeRect(rightX + 0.5, doorY + 0.5, half - 1, doorHeight - 1);
  }
}

