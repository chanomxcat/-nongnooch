import type { IconName } from '@nn/tokens/icon-names';

/** Shared icon sprite from @nn/tokens (copied to /icons.svg). Decorative by default. */
export function Icon({ name, size = 18, className }: { name: IconName; size?: number; className?: string }) {
  return (
    <svg className={`nn-icon ${className ?? ''}`} width={size} height={size} aria-hidden="true" focusable="false">
      <use href={`/icons.svg#${name}`} />
    </svg>
  );
}

export type { IconName };
