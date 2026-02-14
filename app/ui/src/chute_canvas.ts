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
  private cachedWidth = 0;
  private cachedHeight = 0;
  private particles: Array<{
    x: number;
    y: number;
    z: number; // 0 = near, 1 = far
    speed: number;
    size: number; // Random size between 1-3
  }> = [];
  private readonly particleCount = 30;
  private readonly particleSpawnRate = 0.02;

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
    // Initialize particles
    this.particles = [];
    for (let i = 0; i < this.particleCount; i += 1) {
      this.particles.push({
        x: Math.random() * this.cachedWidth,
        y: Math.random() * this.cachedHeight,
        z: Math.random(),
        speed: 0.01 + Math.random() * 0.02,
        size: 1 + Math.random() * 2, // Random size between 1-3
      });
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
    this.doorProgress += (this.doorTarget - this.doorProgress) * this.lerpFactor;
    if (Math.abs(this.doorTarget - this.doorProgress) < 0.001) {
      this.doorProgress = this.doorTarget;
    }
    
    let tunnelPhase = ((now / 1000) * this.tunnelSpeed) % 1;
    if (tunnelPhase < 0) tunnelPhase += 1;
    
    // Update particles
    this.updateParticles();
    
    this.drawFunnelDoors(this.cachedWidth, this.cachedHeight, this.doorProgress, tunnelPhase);
  }
  
  private updateParticles() {
    const width = this.cachedWidth;
    const height = this.cachedHeight;
    
    if (width === 0 || height === 0) {
      return;
    }
    
    for (let i = 0; i < this.particles.length; i += 1) {
      const p = this.particles[i];
      
      // Move particle based on direction
      if (this.pulseDirection === "in") {
        // Flow inward: z increases (gets closer)
        p.z += p.speed;
        if (p.z >= 1) {
          // Reset to back (far)
          p.z = 0;
          p.x = Math.random() * width;
          p.y = Math.random() * height;
        }
      } else {
        // Flow outward: z decreases (gets farther)
        p.z -= p.speed;
        if (p.z <= 0) {
          // Reset to front (near)
          p.z = 1;
          p.x = Math.random() * width;
          p.y = Math.random() * height;
        }
      }
    }
    
    // Spawn new particles occasionally
    if (Math.random() < this.particleSpawnRate && this.particles.length < this.particleCount * 1.5) {
      this.particles.push({
        x: Math.random() * width,
        y: Math.random() * height,
        z: this.pulseDirection === "in" ? 0 : 1,
        speed: 0.01 + Math.random() * 0.02,
        size: 1 + Math.random() * 2, // Random size between 1-3
      });
    }
  }

  private resize() {
    const rect = this.canvas.getBoundingClientRect();
    this.cachedWidth = rect.width;
    this.cachedHeight = rect.height;
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
      
      // Pulse acceleration: speed up near center when sending, expand when receiving
      const normalizedPos = i / (rings - 1);
      let phaseOffset = i * 0.12;
      if (this.pulseDirection === "in") {
        // Accelerate inward (tighter spacing near center)
        phaseOffset *= 1 + normalizedPos * 0.8;
      } else {
        // Expand outward (tighter spacing at edges)
        phaseOffset *= 1 + (1 - normalizedPos) * 0.8;
      }
      
      // Calculate local phase with proper wrapping
      let localPhase: number;
      if (this.pulseDirection === "in") {
        localPhase = ((1 - phase + phaseOffset) % 1 + 1) % 1;
      } else {
        localPhase = ((phase + phaseOffset) % 1 + 1) % 1;
      }
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
    
    // Draw particles
    this.drawParticles(width, height);
  }
  
  private drawParticles(width: number, height: number) {
    const particleColor = this.pulseDirection === "in"
      ? { r: 80, g: 255, b: 255 }  // Blue for sender
      : { r: 255, g: 150, b: 60 }; // Orange for receiver
    
    const centerX = width / 2;
    const centerY = height / 2;
    
    for (const p of this.particles) {
      // Z-depth calculation: z=0 is far (furthest from screen), z=1 is near (closest to screen)
      // For inward flow (blue): z increases (0->1) means moving from edges toward center
      // For outward flow (orange): z decreases (1->0) means moving from center toward edges
      const depthFactor = this.pulseDirection === "in" ? p.z : 1 - p.z;
      
      // Random size between 1-3px (stored per particle)
      const size = p.size * 0.5; // Convert to radius
      
      // Alpha falls off as particle approaches screen (both fade as they get closer)
      // z=0 is far (bright), z=1 is near (dim) -> alpha = 1 - z for both
      const opacity = 1 - p.z;
      
      // Project particle position: for inward flow, move from edges (factor=1) to center (factor=0)
      // For outward flow, move from center (factor=0) to edges (factor=1)
      const projectionFactor = this.pulseDirection === "in" ? (1 - depthFactor) : depthFactor;
      const projectedX = centerX + (p.x - centerX) * projectionFactor;
      const projectedY = centerY + (p.y - centerY) * projectionFactor;
      
      // Draw particle
      this.ctx.fillStyle = `rgba(${particleColor.r}, ${particleColor.g}, ${particleColor.b}, ${opacity})`;
      this.ctx.beginPath();
      this.ctx.arc(projectedX, projectedY, size, 0, Math.PI * 2);
      this.ctx.fill();
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

