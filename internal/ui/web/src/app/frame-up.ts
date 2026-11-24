import type { Up } from '@/components/screen-switch';
import type { Order } from '@/stores/order';
import type { Back } from '@/utils/back';

// frameUp is a page whole's way back: where it was opened from, and a press that keeps that the rows go on there
// (order.comeBack). The back link, the header's link to that list and h all go by it, so that one press marks once
export const frameUp = (back: Back, order: Pick<Order, 'editKept'>): Up => ({
  href: back.href,
  onPress: () => {
    order.editKept(() => ({ pressed: true }));
  },
});
