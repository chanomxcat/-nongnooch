import { Icon } from '@/components/ui/Icon';
import s from '@/components/account/account.module.css';

export function ErrorBox({ text }: { text: string }) {
  return (
    <p className={s.alert} role="alert">
      <Icon name="alert" size={18} />
      <span>{text}</span>
    </p>
  );
}
