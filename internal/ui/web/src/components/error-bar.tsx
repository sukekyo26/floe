import { useStore } from '@/hooks/use-store';
import { notice as pageNotice, type Notice } from '@/stores/notice';

// ErrorBar says the page's one mistake under the bar, on the list and in the frame alike. It stands outside the
// screen's DrawGuard, so that a draw that throws leaves it to say so
export function ErrorBar({ notice = pageNotice }: { readonly notice?: Notice }) {
  const { message } = useStore(notice.said);
  return (
    <p
      id="error"
      role="alert"
      hidden={!message}
      className="mx-4 mt-3 mb-0 rounded-menu bg-danger-soft px-3.5 py-2.5 whitespace-pre-wrap text-danger"
    >
      {message}
    </p>
  );
}
