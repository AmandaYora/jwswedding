import { useParams } from "react-router-dom";
import { QuotationEditorSection } from "@/modules/quotations/components/QuotationEditorSection";

export default function QuotationDetailPage() {
  const { quotationId } = useParams<{ quotationId: string }>();
  if (!quotationId) return <p className="py-10 text-center text-[13px] text-text-secondary">Penawaran tidak ditemukan.</p>;
  return <QuotationEditorSection quotationId={quotationId} />;
}
