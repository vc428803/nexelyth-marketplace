import { useEffect, useMemo, useState, type FormEvent } from "react";
import { ArrowLeft, Clock3, Coins, Gavel, ImageOff, ShieldCheck, Sword, Tag } from "lucide-react";

/**
 * NEXELYTH auction detail visual layer.
 * No API calls and no database writes live here: the existing route supplies
 * listing, bid records and its established submit handler.
 */
export type AuctionDetailListing = {
  id: string;
  item_name: string;
  item_description?: string | null;
  item_image_url: string;
  category?: string | null;
  item_code?: string | null;
  current_price: number;
  starting_price: number;
  min_increment: number;
  status: string;
  end_time: string;
};

export type AuctionDetailBid = {
  id: string;
  amount: number;
  created_at: string;
  bidder_label?: string | null;
};

type Props = {
  listing: AuctionDetailListing;
  bids: AuctionDetailBid[];
  onBack: () => void;
  onPlaceBid?: (amount: number) => void | Promise<void>;
  isSubmitting?: boolean;
  bidError?: string | null;
  currencyLabel?: string;
};

const numberFormat = new Intl.NumberFormat("zh-TW");
const formatAmount = (value: number) => numberFormat.format(value);
const validAmount = (value: number) => Number.isSafeInteger(value) && value > 0;

function formatCountdown(endTime: string, now: number) {
  const expiry = new Date(endTime).getTime();
  if (!Number.isFinite(expiry)) return "時間未設定";
  const seconds = Math.max(0, Math.floor((expiry - now) / 1000));
  if (seconds <= 0) return "已結束";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;
  return `${days > 0 ? `${days}天 ` : ""}${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
}

export default function AuctionDetailView({
  listing,
  bids,
  onBack,
  onPlaceBid,
  isSubmitting = false,
  bidError,
  currencyLabel = "",
}: Props) {
  const [now, setNow] = useState(() => Date.now());
  const [imageFailed, setImageFailed] = useState(false);
  const minBid = useMemo(() => listing.current_price + listing.min_increment, [listing.current_price, listing.min_increment]);
  const [amount, setAmount] = useState(() => String(listing.current_price + listing.min_increment));

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  useEffect(() => setImageFailed(false), [listing.item_image_url]);
  // Keep amount valid when a competing bid changes the current price.
  useEffect(() => {
    setAmount((current) => Number(current) >= minBid ? current : String(minBid));
  }, [minBid]);

  const endMs = new Date(listing.end_time).getTime();
  const active = listing.status.toLowerCase() === "active" && Number.isFinite(endMs) && endMs > now;
  const parsedAmount = Number(amount);
  const canSubmit =
    !!onPlaceBid &&
    active &&
    !isSubmitting &&
    validAmount(parsedAmount) &&
    parsedAmount >= minBid;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!onPlaceBid || !canSubmit) return;

    await onPlaceBid(parsedAmount);
  }
  return (
    <main className="relative isolate min-h-[calc(100dvh-70px)] overflow-hidden bg-[#030b17] px-4 pb-12 pt-6 text-slate-100 sm:px-7 lg:px-10">
      <div aria-hidden="true" className="pointer-events-none absolute inset-0 -z-10 bg-cover bg-center opacity-30"
        style={{ backgroundImage: "linear-gradient(90deg,#030b17cc,#030b1788),url('/nexelyth/auction/nexelyth-auction-hall-bg.png')" }} />
      <div className="mx-auto max-w-[1500px]">
        <button type="button" onClick={onBack} className="mb-5 inline-flex items-center gap-2 text-sm text-cyan-200 hover:text-amber-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-cyan-300">
          <ArrowLeft size={17} /> 返回競拍殿堂
        </button>

        <div className="grid items-stretch gap-6 lg:grid-cols-[minmax(0,1.45fr)_minmax(360px,1fr)]">
          {/* One image only. No thumbnails and no fabricated angles. */}
          <section aria-label="商品大圖" className="relative flex min-h-[380px] items-center justify-center overflow-hidden rounded-2xl border border-amber-500/40 bg-[radial-gradient(ellipse_at_center,#12446f55_0%,#051223_70%)] shadow-[inset_0_0_70px_#062a47,0_15px_45px_#0007] sm:min-h-[530px]">
            <div aria-hidden="true" className="pointer-events-none absolute inset-x-[15%] bottom-6 h-20 rounded-[50%] border border-cyan-400/40 bg-cyan-500/10 blur-md" />
            {listing.item_image_url?.trim() && !imageFailed ? (
              <img src={listing.item_image_url} alt={listing.item_name} onError={() => setImageFailed(true)}
                className="relative z-10 max-h-[68vh] w-full max-w-[700px] object-contain p-8 drop-shadow-[0_0_30px_#2a9bff66]" />
            ) : (
              <div className="flex flex-col items-center gap-3 text-slate-400"><ImageOff size={42} /><p>圖片尚未提供或載入失敗</p></div>
            )}
          </section>

          <section aria-labelledby="auction-item-name" className="rounded-2xl border border-amber-500/50 bg-[#061526]/95 p-5 shadow-[0_18px_70px_#0009] sm:p-8">
            <div className="mb-3 flex flex-wrap items-center gap-3">
              <h1 id="auction-item-name" className="text-3xl font-bold tracking-wider text-amber-200 sm:text-4xl">{listing.item_name}</h1>
            </div>
            <p className="mb-6 leading-7 text-slate-300">{listing.item_description || "暫無物品描述"}</p>
            <dl className="mb-6 space-y-3 border-y border-cyan-900/80 py-5 text-sm">
              <div className="flex items-center justify-between gap-4"><dt className="flex items-center gap-2 text-cyan-200"><Tag size={16} />分類</dt><dd>{listing.category || "未分類"}</dd></div>
              {listing.item_code && <div className="flex items-center justify-between gap-4"><dt className="flex items-center gap-2 text-cyan-200"><Sword size={16} />物品編號</dt><dd className="font-mono">{listing.item_code}</dd></div>}
              <div className="flex items-center justify-between gap-4"><dt className="flex items-center gap-2 text-cyan-200"><ShieldCheck size={16} />拍賣狀態</dt><dd className={active ? "text-emerald-300" : "text-rose-300"}>{active ? "● 進行中" : "已結束或未開放"}</dd></div>
            </dl>
            <div className="space-y-4 rounded-xl border border-cyan-800/60 bg-[#03111f]/80 p-5">
              <div className="flex items-center justify-between gap-3"><span className="flex items-center gap-2 text-slate-300"><Clock3 size={17} />剩餘時間</span><strong className="text-xl font-bold tabular-nums text-rose-400">{formatCountdown(listing.end_time, now)}</strong></div>
              <div className="flex items-center justify-between gap-3"><span className="flex items-center gap-2 text-slate-300"><Coins size={17} />目前出價</span><strong className="text-3xl font-bold tabular-nums text-amber-300">{currencyLabel}{formatAmount(listing.current_price)}</strong></div>
              <div className="flex items-center justify-between text-sm"><span className="text-slate-400">起標價格</span><span>{currencyLabel}{formatAmount(listing.starting_price)}</span></div>
              <div className="flex items-center justify-between text-sm"><span className="text-slate-400">最低加價金額</span><span>{currencyLabel}{formatAmount(listing.min_increment)}</span></div>
            </div>

            <form onSubmit={submit} className="mt-6 space-y-3">
              <label htmlFor="nexelyth-bid-amount" className="block text-sm font-semibold text-amber-200">出價金額</label>
              <div className="flex flex-col gap-3 sm:flex-row">
                <input id="nexelyth-bid-amount" type="number" step="1" min={minBid} inputMode="numeric" value={amount}
                  onChange={(e) => setAmount(e.target.value)} disabled={!onPlaceBid || !active || isSubmitting}
                  className="min-w-0 flex-1 rounded-lg border border-cyan-700 bg-[#071d31] px-4 py-3 text-lg tabular-nums text-white outline-none focus:border-amber-400 disabled:opacity-50" />
                <button
                  type="submit"
                  disabled={!canSubmit}
                  className="inline-flex items-center justify-center gap-2 rounded-lg
             bg-gradient-to-b from-amber-300 to-amber-600
             px-7 py-3 font-bold text-[#09111d]
             hover:brightness-110
             disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Gavel size={19} />
                  {isSubmitting
                    ? "提交中…"
                    : !onPlaceBid
                      ? "出價功能待接入"
                      : "出價競標"}
                </button>
              </div>
              <p className="text-xs text-slate-400">最低有效出價：{currencyLabel}{formatAmount(minBid)}。最終金額及資格由後端重新驗證。</p>
              {bidError && <p role="alert" className="text-sm text-rose-300">{bidError}</p>}
            </form>
          </section>
        </div>

        <section aria-labelledby="bid-history-heading" className="mt-6 rounded-2xl border border-amber-500/40 bg-[#061526]/95 p-5 sm:p-7">
          <div className="mb-4 flex items-center gap-3"><h2 id="bid-history-heading" className="text-xl font-bold text-amber-200">出價紀錄</h2><span className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-300">{bids.length} 筆</span></div>
          {bids.length === 0 ? (
            <div className="flex min-h-32 flex-col items-center justify-center gap-3 border-t border-cyan-900/60 text-slate-400"><Gavel size={30} /><p>目前還沒有人出價，成為第一位出價的冒險者吧！</p></div>
          ) : (
            <div className="overflow-x-auto"><table className="w-full min-w-[490px] text-left text-sm"><thead className="border-b border-cyan-900/70 text-cyan-200"><tr><th className="py-3">出價時間</th><th className="py-3">出價者</th><th className="py-3 text-right">出價金額</th></tr></thead><tbody>{bids.map((bid) => <tr key={bid.id} className="border-b border-cyan-900/40"><td className="py-3">{new Date(bid.created_at).toLocaleString("zh-TW")}</td><td className="py-3">{bid.bidder_label || "冒險者"}</td><td className="py-3 text-right font-semibold text-amber-200">{currencyLabel}{formatAmount(bid.amount)}</td></tr>)}</tbody></table></div>
          )}
        </section>
      </div>
    </main>
  );
}
