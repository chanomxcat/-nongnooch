import { OrderResult } from '@/components/checkout/OrderResult';

export const metadata = { robots: { index: false } };

export default function ResultPage({ params }: { params: { orderId: string } }) {
  return <OrderResult orderId={params.orderId} />;
}
