import { useEffect, useRef, type FC } from "react";
import * as THREE from "three";

interface ParticleWaveProps {
  className?: string;
}

const PARTICLE_VERTEX = `
  attribute float scale;
  uniform float uTime;
  void main() {
    vec3 p = position;
    float s = scale;
    p.y += (sin(p.x + uTime) * 0.5) + (cos(p.y + uTime) * 0.1) * 2.0;
    p.x += (sin(p.y + uTime) * 0.5);
    s += (sin(p.x + uTime) * 0.5) + (cos(p.y + uTime) * 0.1) * 2.0;
    vec4 mvPosition = modelViewMatrix * vec4(p, 1.0);
    gl_PointSize = s * 15.0 * (1.0 / -mvPosition.z);
    gl_Position = projectionMatrix * mvPosition;
  }
`;

const PARTICLE_FRAGMENT = `
  uniform vec3 uColor;
  void main() {
    gl_FragColor = vec4(uColor, 0.5);
  }
`;

const ParticleWave: FC<ParticleWaveProps> = ({ className = "" }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const sceneRef = useRef<{
    scene: THREE.Scene;
    camera: THREE.PerspectiveCamera;
    renderer: THREE.WebGLRenderer;
    particles: THREE.Points;
    particleMaterial: THREE.ShaderMaterial;
    animationId: number | null;
    mouse: THREE.Vector2;
  } | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const reducedMotion =
      typeof window.matchMedia === "function"
        ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
        : false;

    const getSize = () => {
      const parent = canvas.parentElement;
      const width = Math.max(1, parent?.clientWidth || canvas.clientWidth || window.innerWidth);
      const height = Math.max(1, parent?.clientHeight || canvas.clientHeight || window.innerHeight);
      return { width, height };
    };

    // Login promo is dark-only (no teal/green tint).
    const backgroundColor = new THREE.Color(0x0a0a0a);
    const particleColor = new THREE.Vector3(1.0, 1.0, 1.0);

    const { width: winWidth, height: winHeight } = getSize();
    const aspectRatio = winWidth / winHeight;

    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({
        canvas,
        antialias: true,
        alpha: true,
      });
    } catch {
      // jsdom / environments without WebGL — leave empty canvas.
      return;
    }

    const camera = new THREE.PerspectiveCamera(75, aspectRatio, 0.01, 1000);
    camera.position.set(0, 6, 5);

    const scene = new THREE.Scene();

    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.setSize(winWidth, winHeight, false);
    renderer.setClearColor(backgroundColor, 1);

    const gap = 0.3;
    const amountX = 200;
    const amountY = 200;
    const particleNum = amountX * amountY;
    const particlePositions = new Float32Array(particleNum * 3);
    const particleScales = new Float32Array(particleNum);

    let i = 0;
    let j = 0;
    for (let ix = 0; ix < amountX; ix++) {
      for (let iy = 0; iy < amountY; iy++) {
        particlePositions[i] = ix * gap - (amountX * gap) / 2;
        particlePositions[i + 1] = 0;
        particlePositions[i + 2] = iy * gap - (amountX * gap) / 2;
        particleScales[j] = 1;
        i += 3;
        j++;
      }
    }

    const particleGeometry = new THREE.BufferGeometry();
    particleGeometry.setAttribute("position", new THREE.BufferAttribute(particlePositions, 3));
    particleGeometry.setAttribute("scale", new THREE.BufferAttribute(particleScales, 1));

    const particleMaterial = new THREE.ShaderMaterial({
      transparent: true,
      vertexShader: PARTICLE_VERTEX,
      fragmentShader: PARTICLE_FRAGMENT,
      uniforms: {
        uTime: { value: 0 },
        uColor: { value: particleColor },
      },
    });

    const particles = new THREE.Points(particleGeometry, particleMaterial);
    scene.add(particles);

    const mouse = new THREE.Vector2(-10, -10);

    sceneRef.current = {
      scene,
      camera,
      renderer,
      particles,
      particleMaterial,
      animationId: null,
      mouse,
    };

    const animate = () => {
      if (!sceneRef.current) return;
      const { scene: sc, camera: cam, renderer: ren, particleMaterial: mat } = sceneRef.current;
      mat.uniforms.uTime.value += 0.05;
      cam.lookAt(sc.position);
      ren.render(sc, cam);
      sceneRef.current.animationId = requestAnimationFrame(animate);
    };

    const handleResize = () => {
      if (!sceneRef.current) return;
      const { camera: cam, renderer: ren } = sceneRef.current;
      const { width, height } = getSize();
      cam.aspect = width / height;
      cam.updateProjectionMatrix();
      ren.setSize(width, height, false);
    };

    const handleMouseMove = (e: MouseEvent) => {
      if (!sceneRef.current) return;
      const { width, height } = getSize();
      sceneRef.current.mouse.x = (e.clientX / width) * 2 - 1;
      sceneRef.current.mouse.y = -(e.clientY / height) * 2 + 1;
    };

    if (!reducedMotion) {
      animate();
    } else {
      camera.lookAt(scene.position);
      renderer.render(scene, camera);
    }

    const resizeObserver =
      typeof ResizeObserver !== "undefined" ? new ResizeObserver(() => handleResize()) : null;
    if (resizeObserver && canvas.parentElement) {
      resizeObserver.observe(canvas.parentElement);
    }

    window.addEventListener("mousemove", handleMouseMove);

    return () => {
      if (sceneRef.current?.animationId) {
        cancelAnimationFrame(sceneRef.current.animationId);
      }
      resizeObserver?.disconnect();
      window.removeEventListener("mousemove", handleMouseMove);

      if (sceneRef.current) {
        const { scene: sc, renderer: ren, particles: pts } = sceneRef.current;
        sc.remove(pts);
        if (pts.geometry) pts.geometry.dispose();
        if (pts.material) {
          if (Array.isArray(pts.material)) {
            pts.material.forEach((material) => material.dispose());
          } else {
            pts.material.dispose();
          }
        }
        ren.dispose();
      }
      sceneRef.current = null;
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className={className ? `particle-wave ${className}` : "particle-wave"}
      aria-hidden
    />
  );
};

export { ParticleWave };
