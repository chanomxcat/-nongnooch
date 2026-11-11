'use client';

import { useEffect, useRef, useState, type ElementType, type ReactNode, type CSSProperties } from 'react';

/** Motion G: fade/slide in once when 20% visible. CSS turns it off under prefers-reduced-motion. */
export function Reveal({
  as: Tag = 'div',
  className,
  delay = 0,
  children,
}: {
  as?: ElementType;
  className?: string;
  delay?: number;
  children: ReactNode;
}) {
  const ref = useRef<HTMLElement>(null);
  const [inView, setInView] = useState(false);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof IntersectionObserver === 'undefined') {
      setInView(true);
      return;
    }
    const io = new IntersectionObserver(
      ([e]) => {
        if (e.isIntersecting) {
          setInView(true);
          io.disconnect();
        }
      },
      { threshold: 0.1, rootMargin: "0px 0px -5% 0px" },
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);
  const style: CSSProperties | undefined = delay ? { transitionDelay: `${delay}ms` } : undefined;
  return (
    <Tag ref={ref} className={`nn-reveal ${inView ? 'is-in' : ''} ${className ?? ''}`} style={style}>
      {children}
    </Tag>
  );
}
