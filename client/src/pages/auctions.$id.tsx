import { useCallback, useEffect, useState } from "react";
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
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [bidError, setBidError] = useState("");


  const loadAuction = useCallback(async (signal: AbortSignal) => {
    setLoading(true);
    setError("");

    try {
      const [listingRes, bidsRes] = await Promise.all([
        fetch(apiUrl(`/auctions/${encodeURIComponent(id)}`), {
          credentials: "include",
          signal,
        }),
        fetch(apiUrl(`/auctions/${encodeURIComponent(id)}/bids`), {
          credentials: "include",
          signal,
        }),
      ]);

      if (!listingRes.ok) {
        throw new Error(`商品查詢失敗 (${listingRes.status})`);
      }

      const listingPayload =
        (await listingRes.json()) as ListingResponse;

      if (!listingPayload.data) {
        throw new Error("找不到拍賣商品");
      }

      let bidRecords: AuctionDetailBid[] = [];

      if (bidsRes.ok) {
        const bidPayload =
          (await bidsRes.json()) as BidResponse;

        bidRecords = (
          Array.isArray(bidPayload.data) ? bidPayload.data : []
        ).map((bid) => ({
          id: bid.id,
          amount: bid.bid_amount,
          bidder_label: bid.bidder_id
            ? `玩家 ${bid.bidder_id.slice(0, 8)}…`
            : "冒險者",
          created_at: bid.created_at,
        }));
      }

      if (signal.aborted) return;

      setListing(listingPayload.data);
      setBids(bidRecords);
    } catch (err) {
      if (signal.aborted) return;

      console.error("Failed to fetch auction details", err);
      setError(
        err instanceof Error
          ? err.message
          : "拍賣資料載入失敗",
      );
    } finally {
      if (!signal.aborted) {
        setLoading(false);
      }
    }
  }, [id]);

  useEffect(() => {
    const controller = new AbortController();

    setListing(null);
    setBids([]);

    void loadAuction(controller.signal);

    return () => controller.abort();
  }, [loadAuction]);


  // 訂閱拍賣 SSE 即時事件
  useEffect(() => {
    let refreshController: AbortController | null = null;
    let connectedOnce = false;

    const eventSource = new EventSource(
      apiUrl(`/auctions/${encodeURIComponent(id)}/stream`),
      { withCredentials: true },
    );

    // SSE 首次連線及斷線重連
    eventSource.onopen = () => {
      console.log("Auction SSE connected:", id);

      // 重連後補回可能遺失的事件
      if (connectedOnce) {
        refreshController?.abort();
        refreshController = new AbortController();
        void loadAuction(refreshController.signal);
      }

      connectedOnce = true;
    };

    // 收到出價更新事件
    eventSource.addEventListener("auction.bid.updated", (event) => {
      console.log("Auction bid updated:", event.data);

      refreshController?.abort();
      refreshController = new AbortController();

      void loadAuction(refreshController.signal);
    });

    eventSource.onerror = () => {
      console.warn("Auction SSE connection interrupted");
    };

    return () => {
      eventSource.close();
      refreshController?.abort();
    };
  }, [id, loadAuction]);


  // 提交競標出價
  async function handlePlaceBid(amount: number) {
    if (isSubmitting) return;

    setIsSubmitting(true);
    setBidError("");

    try {
      const response = await fetch(
        apiUrl(`/auctions/${encodeURIComponent(id)}/bids`),
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            bid_amount: amount,
          }),
        },
      );

      const result = (await response.json()) as {
        message?: string;
        error?: string;
      };

      if (!response.ok) {
        throw new Error(
          result.error || `出價失敗 (${response.status})`,
        );
      }

      console.log("Bid placed successfully:", result.message);

      // 通知 Navbar 重新查詢玩家錢包餘額
      window.dispatchEvent(new Event("wallet:updated"));

      // 主動更新拍賣資料
      const controller = new AbortController();
      await loadAuction(controller.signal);
    } catch (err) {
      console.error("Place bid failed:", err);

      setBidError(
        err instanceof Error
          ? err.message
          : "出價失敗，請稍後再試",
      );
    } finally {
      setIsSubmitting(false);
    }
  }


  if (loading) return <div className="min-h-screen bg-[#030b17] pt-24 text-center text-cyan-200">正在載入拍賣商品…</div>;
  if (error) return <div role="alert" className="min-h-screen bg-[#030b17] pt-24 text-center text-red-300">{error}</div>;
  if (!listing) return <div className="min-h-screen bg-[#030b17] pt-24 text-center text-slate-200">找不到拍賣商品</div>;

  return (
    <AuctionDetailView
      listing={listing}
      bids={bids}
      onBack={() => navigate({ to: "/Aution" })}
      onPlaceBid={handlePlaceBid}
      isSubmitting={isSubmitting}
      bidError={bidError}
    />
  );
}

export const Route = createFileRoute("/auctions/$id")({
  component: AuctionDetailPage,
});
