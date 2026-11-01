import { useEffect, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { apiUrl } from "../config/api";
import AuctionDetailView, {
  type AuctionDetailBid,
  type AuctionDetailListing,
} from "../components/auction/AuctionDetailView";

type ListingResponse = { data?: AuctionDetailListing | null };
type BidResponse = {
  data?: Array<{
    id: string;
    bid_amount: number;
    bidder_id?: string | null;
    created_at: string;
  }>;
};

function AuctionDetailPage() {
  const { id } = Route.useParams();
  const navigate = useNavigate();
  const [listing, setListing] = useState<AuctionDetailListing | null>(null);
  const [bids, setBids] = useState<AuctionDetailBid[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();

    async function load() {
      setLoading(true);
      setError("");
      setListing(null);
      setBids([]);
      try {
        const [listingRes, bidsRes] = await Promise.all([
          fetch(apiUrl(`/auctions/${encodeURIComponent(id)}`), {
            credentials: "include",
            signal: controller.signal,
          }),
          fetch(apiUrl(`/auctions/${encodeURIComponent(id)}/bids`), {
            credentials: "include",
            signal: controller.signal,
          }),
        ]);
        if (!listingRes.ok) throw new Error(`商品查詢失敗 (${listingRes.status})`);
        const listingPayload = (await listingRes.json()) as ListingResponse;
        if (!listingPayload.data) throw new Error("找不到拍賣商品");

        let bidRecords: AuctionDetailBid[] = [];
        if (bidsRes.ok) {
          const bidPayload = (await bidsRes.json()) as BidResponse;
          bidRecords = (Array.isArray(bidPayload.data) ? bidPayload.data : []).map(
            (bid) => ({
              id: bid.id,
              amount: bid.bid_amount,
              // API only provides bidder_id at present; show an abbreviated ID,
              // not a fabricated player's display name.
              bidder_label: bid.bidder_id
                ? `玩家 ${bid.bidder_id.slice(0, 8)}…`
                : "冒險者",
              created_at: bid.created_at,
            }),
          );
        }
        if (controller.signal.aborted) return;
        setListing(listingPayload.data);
        setBids(bidRecords);
      } catch (err) {
        if (controller.signal.aborted) return;
        console.error("Failed to fetch auction details", err);
        setError(err instanceof Error ? err.message : "拍賣資料載入失敗");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }

    void load();
    return () => controller.abort();
  }, [id]);

  if (loading) return <div className="min-h-screen bg-[#030b17] pt-24 text-center text-cyan-200">正在載入拍賣商品…</div>;
  if (error) return <div role="alert" className="min-h-screen bg-[#030b17] pt-24 text-center text-red-300">{error}</div>;
  if (!listing) return <div className="min-h-screen bg-[#030b17] pt-24 text-center text-slate-200">找不到拍賣商品</div>;

  return (
    <AuctionDetailView
      listing={listing}
      bids={bids}
      onBack={() => navigate({ to: "/Aution" })}
      // No verified bid submission endpoint/handler was provided in the
      // existing page; the UI disables bidding until that integration exists.
    />
  );
}

export const Route = createFileRoute("/auctions/$id")({
  component: AuctionDetailPage,
});
