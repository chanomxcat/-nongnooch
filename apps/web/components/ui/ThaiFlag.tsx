/** Thai national flag (image asset, so its colours are not design tokens). */
export function ThaiFlag({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={(size * 2) / 3} viewBox="0 0 9 6" aria-hidden="true" style={{ borderRadius: 2, flex: 'none' }}>
      <rect width="9" height="6" fill="#A51931" />
      <rect y="1" width="9" height="4" fill="#F4F5F8" />
      <rect y="2" width="9" height="2" fill="#2D2A4A" />
    </svg>
  );
}
