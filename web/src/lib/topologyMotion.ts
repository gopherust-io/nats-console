import { useReducedMotion, type Transition, type Variants } from "motion/react";

const EASE_OUT: [number, number, number, number] = [0.22, 1, 0.36, 1];

export const TOPOLOGY_FLOW_VISIBLE = 8;

export function useTopologyMotion() {
  const reduceMotion = Boolean(useReducedMotion());

  const transition: Transition = reduceMotion
    ? { duration: 0 }
    : { duration: 0.32, ease: EASE_OUT };

  const spring: Transition = reduceMotion
    ? { duration: 0 }
    : { type: "spring", stiffness: 380, damping: 28, mass: 0.8 };

  const softSpring: Transition = reduceMotion
    ? { duration: 0 }
    : { type: "spring", stiffness: 260, damping: 32 };

  const collapseTransition: Transition = reduceMotion
    ? { duration: 0 }
    : { duration: 0.3, ease: EASE_OUT };

  const pathDraw: Transition = reduceMotion
    ? { duration: 0 }
    : { duration: 0.7, ease: EASE_OUT };

  const signalTravel = reduceMotion
    ? { duration: 0 }
    : { duration: 2.4, ease: "linear" as const, repeat: Infinity, repeatType: "loop" as const };

  const orbit = reduceMotion
    ? { duration: 0 }
    : { duration: 28, ease: "linear" as const, repeat: Infinity };

  const panelVariants: Variants = {
    hidden: reduceMotion ? { opacity: 0 } : { opacity: 0, y: 12, scale: 0.98 },
    visible: { opacity: 1, y: 0, scale: 1 },
    exit: reduceMotion ? { opacity: 0 } : { opacity: 0, y: 8, scale: 0.985 },
  };

  const listVariants: Variants = {
    collapsed: {},
    open: {
      transition: reduceMotion
        ? { staggerChildren: 0, delayChildren: 0 }
        : { staggerChildren: 0.045, delayChildren: 0.04 },
    },
  };

  const itemVariants: Variants = {
    collapsed: reduceMotion ? { opacity: 0 } : { opacity: 0, x: -8, y: 4 },
    open: { opacity: 1, x: 0, y: 0 },
    hidden: reduceMotion ? { opacity: 0 } : { opacity: 0, x: -8, y: 4 },
    visible: { opacity: 1, x: 0, y: 0 },
  };

  const statsVariants: Variants = {
    hidden: {},
    visible: {
      transition: reduceMotion
        ? { staggerChildren: 0 }
        : { staggerChildren: 0.07, delayChildren: 0.05 },
    },
  };

  const statItemVariants: Variants = {
    hidden: reduceMotion ? { opacity: 0 } : { opacity: 0, y: 14, scale: 0.96 },
    visible: { opacity: 1, y: 0, scale: 1 },
  };

  return {
    reduceMotion,
    transition,
    spring,
    softSpring,
    collapseTransition,
    pathDraw,
    signalTravel,
    orbit,
    panelVariants,
    listVariants,
    itemVariants,
    statsVariants,
    statItemVariants,
    explorerInitial: reduceMotion ? false : ({ opacity: 0, y: 16 } as const),
    explorerAnimate: { opacity: 1, y: 0 } as const,
  };
}
