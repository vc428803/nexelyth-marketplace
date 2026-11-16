# NEXELYTH Marketplace

> 虛擬資產競拍與交易平台｜React + TypeScript + Go + Gin + Neon PostgreSQL  
> **開發快照：2026-11-17**｜[GitHub Repository](https://github.com/vc428803/nexelyth-marketplace)

NEXELYTH Marketplace 是 NEXELYTH 生態系的 Web 交易平台，負責玩家登入、背包資產、拍賣刊登、競標資金保留與到期結算。後端以 PostgreSQL Transaction 與 Row-Level Locking 維持金流及資產一致性，前端提供奇幻風格的拍賣介面。

**目前進度：後端拍賣主流程已完成並通過基本端對端驗證；正在開發「拍賣專用 SSE 即時通知」，尚未完成 Route／事件發布／React 訂閱整合。**

## 01. 快速掌握目前進度

| 項目 | 目前狀態 | 備註 |
| --- | --- | --- |
| 使用者註冊／登入 | 已實作 | Email、bcrypt、JWT、HttpOnly Cookie |
| Marketplace Inventory | 已實作 | 物品定義、背包實例、所有權及狀態驗證 |
| 拍賣刊登 | 已實作並測試 | 不可重複上架同一件 `LISTED` 物品 |
| 出價 `PlaceBid()` | 已實作並測試 | 最低加價、截止時間、錢包鎖定 |
| `BID_HOLD`／`BID_RELEASE` | 已實作 | 保留、釋放、交易流水；多競標者情境仍需完整測試 |
| `SettleAuction()` | 已實作並測試 | 成交付款、物品移轉、無人出價流標 |
| 冪等性 | 基本重試測試通過 | 已完成拍賣重呼叫不重複產生 `SALE` 流水；尚待並行測試 |
| Auction Background Worker | 已實作並測試 | 啟動即掃描、之後每 30 分鐘掃描 |
| Graceful Shutdown | 閒置狀態測試通過 | Ctrl+C → HTTP Shutdown → Worker 停止；交易中斷情境待測 |
| 前端拍賣狀態顯示 | 已實作 | 競標中／待結算／已成交／已流標 |
| 前端出價按鈕接 API | **尚未完成** | 詳情 Route 未傳入 `onPlaceBid` |
| 商品展示 SSE | 已有基礎實作 | `/products/stream`，與拍賣事件不同 |
| **拍賣 SSE EventHub** | **已新增、待整合** | `auction_event_hub.go`、`auction_sse.go` 已在 GitHub |
| Auction SSE Route／事件推播 | **下一步** | 尚未註冊 `/auctions/:id/stream`，尚未從交易 Commit 後發布 |
| Unity VR 資產同步 | 規劃中 | 跨平台資產／領取流程尚未實作 |

> 「已實作」與「測試通過」分開判定；目前尚未完成完整的壓力、並行出價、故障注入與部署驗收。

## 02. 技術架構

| 層級 | 技術 |
| --- | --- |
| Web Client | React、TypeScript、Vite、Tailwind CSS |
| UI / Routing | Lucide React、TanStack Router |
| Backend | Go、Gin |
| Authentication | JWT（HS256）、bcrypt、HttpOnly Cookie |
| Database | Neon PostgreSQL、pgx/v5、pgxpool |
| Schema Migration | golang-migrate；Migration 至 `000015` |
| Background Job | Go Auction Worker、`context.Context`、`WaitGroup` |
| Realtime（現有） | WebSocket `/ws`、Products SSE `/products/stream` |
| Realtime（開發中） | Auction SSE、`AuctionEventHub` |

```text
React / TypeScript (Web Client)
            |
       REST API / Cookie
            v
      Go / Gin Server
            |
   +--------+---------+-----------------+
   |                  |                 |
 Auth / Inventory   Auction / Wallet   Products SSE / WebSocket
                      |
                Repository Layer
                      |
              pgx / PostgreSQL Tx
                      |
                Neon PostgreSQL
                      ^
                      |
             Auction Worker (Go)
             啟動即掃描 + 每 30 分鐘

[開發中] AuctionEventHub → Auction SSE → React EventSource
```

## 03. 核心業務流程

### A. 刊登拍賣

1. 由 JWT Cookie 識別玩家。
2. 鎖定背包物品並驗證持有者、可交易性與 `AVAILABLE` 狀態。
3. 在同一個資料庫 Transaction 中建立 `auction_listings`，將物品改為 `LISTED`。
4. 交易失敗時回滾，避免拍賣與背包狀態不一致。

### B. 競標與資金保留（`PlaceBid`）

```text
鎖定 auction_listings (FOR UPDATE)
  → 檢查 active、未截止、非賣家本人、符合最小加價
  → 鎖定相關 wallets（固定排序降低死鎖風險）
  → 若已有最高出價：釋放先前資金 BID_RELEASE
  → 扣除新出價者可用餘額
  → 新增 auction_bids、auction_fund_holds (HELD)
  → 寫入 wallet_transactions (BID_HOLD)
  → 更新最高出價者與 current_price
  → Commit
```

### C. 截止與成交／流標（`SettleAuction`）

| 情境 | 結果 |
| --- | --- |
| 有得標者，資金／物品驗證成功 | 保留資金 `SETTLED`、賣家收到 `SALE` 流水、物品轉給買家、拍賣 `completed` |
| 無人出價且沒有異常保留資金 | 拍賣 `expired`、物品回復 `AVAILABLE`、所有權不變 |
| 已為 `completed` 或 `expired` | 直接返回，避免再次付款或交付 |
| 驗證或交易失敗 | 返回錯誤，未提交交易不可留下部分狀態 |

`SettleAuction()` 會先鎖定拍賣，再驗證資金保留、物品與賣家錢包。2026-11 的修正已將「流標時不能存在 `HELD`」的檢查移入**無人出價**分支，避免正常成交被錯誤阻擋。

### D. 背景自動結算（Auction Worker）

- Go Server 啟動時先補掃一次到期且仍為 `active` 的拍賣。
- 之後每 **30 分鐘**執行一次保底掃描；每批最多 **100 筆**。
- 逐筆呼叫既有 `repository.SettleAuction()`；單筆失敗不停止其他筆。
- 收到停止訊號會退出迴圈，使用 `WaitGroup` 等候停止，再關閉 DB Pool。
- **不是精確到截止秒數的即時結算**：實測在截止後約 17～21 分鐘由下次掃描處理。

## 04. 三個重要資料概念

| 資料表 | 核心用途 |
| --- | --- |
| `auction_listings` | 拍賣狀態、截止時間、最高價與最高出價者 |
| `auction_fund_holds` | 追蹤最高出價資金是否 `HELD`、`RELEASED`、`SETTLED` |
| `inventory_items` | 實際物品持有人及 `AVAILABLE`／`LISTED` 狀態 |

相關表：`users`、`item_definitions`、`auction_bids`、`wallets`、`wallet_transactions`。

| 狀態類別 | 值 | 說明 |
| --- | --- | --- |
| Auction | `active` | 可出價（仍需確認未截止） |
| Auction | `completed` | 已成交並完成結算 |
| Auction | `expired` | 無人得標，已流標 |
| Fund Hold | `HELD` | 出價資金保留中 |
| Fund Hold | `RELEASED` | 前任最高出價資金已釋放 |
| Fund Hold | `SETTLED` | 保留資金已結算 |
| Inventory | `AVAILABLE` | 可使用／可再次刊登 |
| Inventory | `LISTED` | 已刊登拍賣 |

## 05. 已完成的整合測試（2026-11-15）

在 Neon 的獨立測試 Branch `marketplace-auction-testing` 進行，以下為**當時測試記錄**，非對目前 Branch 是否仍存在的保證。

| 測試 | 已觀察到的結果 | 判定 |
| --- | --- | --- |
| 正常出價 | 價格 `1000 → 1100 G`，買家錢包 `1600 → 500 G` | 通過 |
| 資金凍結 | 新增 `HELD 1100 G` | 通過 |
| Worker 自動流標 | 無人出價拍賣更新為 `expired` | 通過 |
| 流標物品歸還 | 原賣家仍為持有者，物品 `AVAILABLE` | 通過 |
| Worker 自動成交 | Log `success=1 failed=0` | 通過 |
| 成交資料一致性 | `completed`、`SETTLED`、物品改由得標者持有 | 通過 |
| 賣家收入流水 | `SALE +1100 G`，流水 `balance_after=1000000` | 通過 |
| 已成交重複結算 | 再次呼叫後 `SALE` 紀錄仍為 1 筆 | 基本重試通過 |
| Graceful Shutdown | Ctrl+C 後 HTTP 及閒置 Worker 正常停止 | 通過 |

關鍵測試拍賣：`c17da47a-601b-4c0d-acfd-d7ec60476316`。2026-11-15 **14:47:39（台灣時間）**，Worker Log 顯示自動成交結算成功；Neon 查詢確認款項狀態與物品所有權一致。

仍待測：多使用者並行競標、並行重複結算、交易中斷回滾、舊出價者資金釋放的完整整合驗證、錢包目前餘額獨立核對。

## 06. API 與即時通訊

| Method | Endpoint | 功能／進度 |
| --- | --- | --- |
| `POST` | `/auth/register` | 使用者註冊 |
| `POST` | `/auth/login` | 登入並設定 Cookie |
| `GET` | `/auth/me` | 目前使用者 |
| `GET` | `/api/wallet` | 查詢錢包 |
| `GET` | `/api/inventory/me` | 查詢背包 |
| `POST` | `/auctions` | 建立拍賣 |
| `GET` | `/auctions` | 拍賣列表 |
| `GET` | `/auctions/:id` | 拍賣詳情 |
| `POST` | `/auctions/:id/bids` | 出價（由後端驗證） |
| `GET` | `/auctions/:id/bids` | 出價紀錄 |
| `GET` | `/products/stream` | 現有商品 SSE |
| `GET` | `/auctions/:id/stream` | **規劃中；尚未註冊 Router** |

### 拍賣 SSE：目前做到哪裡？

已新增並提交：

- `server/internal/stream/auction_event_hub.go`：依拍賣 ID 管理 Subscribe／Unsubscribe／Publish，使用 `sync.RWMutex` 與 Buffered Channel。
- `server/internal/handlers/auction_sse.go`：SSE Headers、`Flush()`、25 秒 Ping、客戶端斷線清理。

**尚未接通：** `main.go` 建立單一共用 Hub、Routes 依賴傳遞、註冊 Stream Endpoint、`PlaceBid`／Worker Commit 後發布事件、React `EventSource` 訂閱。現階段不要把上述 Endpoint 當成已可使用。

預計事件：`auction.bid.updated`、`auction.completed`、`auction.expired`。SSE 只通知資料改變，資料庫仍是唯一可信的交易狀態來源；斷線重連後需重抓最新狀態。

## 07. 專案結構與重要入口

```text
nexelyth-marketplace/
├── client/
│   └── src/
│       ├── components/auction/AuctionDetailView.tsx
│       └── pages/auctions.$id.tsx
├── server/
│   ├── cmd/
│   │   ├── api/main.go                  # HTTP Server、Worker、Shutdown
│   │   ├── seed-wallets/main.go
│   │   └── settle-auction/main.go       # 手動結算工具，需明確 -execute
│   ├── internal/
│   │   ├── app/routes.go                # 所有路由的中央註冊
│   │   ├── app/routes/auction.go        # Auction Routes
│   │   ├── handlers/auction.go          # CreateAuction／PlaceBid HTTP Handler
│   │   ├── handlers/auction_sse.go      # SSE Handler（未接 Router）
│   │   ├── repository/auction.go        # PlaceBid／SettleAuction
│   │   ├── stream/auction_event_hub.go  # SSE Event Hub（未接交易事件）
│   │   └── worker/auction_worker.go     # 定期結算
│   ├── migrations/                      # 000001 ～ 000015
│   └── go.mod
├── docs/
│   ├── ROADMAP.md
│   └── architecture/001-cross-platform.md
└── README.md
```

前端詳情元件 `AuctionDetailView.tsx` 已能呈現四種狀態；但 `pages/auctions.$id.tsx` 目前只在載入時取得拍賣與出價紀錄，**尚未傳入 `onPlaceBid`**，也尚未接入 SSE。後端 API 出價成功不代表網頁出價按鈕已可使用。

## 08. 本機啟動與安全注意事項

### Backend

```bash
cd server
go build ./...
go run ./cmd/api
```

環境設定由 `server/internal/config/config.go` 讀取，至少需要：

```dotenv
POSTGRES_URL=<your-neon-postgresql-connection-string>
POSTGRES_PORT=3000
JWT_SECRET=<your-local-secret>
```

> `POSTGRES_PORT` 在目前程式中是 Go HTTP Server 的監聽 Port（例如 `3000`），不是 PostgreSQL 資料庫連線埠。實際 DB 連線來自 `POSTGRES_URL`。

### Frontend

```bash
cd client
npm install
npm run dev
```

### 手動結算工具

```bash
cd server
go run ./cmd/settle-auction -listing=<auction-uuid>
# 上一行為 Dry Run；真正執行須額外加入 -execute
```

**不要在 production 使用測試交易或直接修改錢包餘額。** 先建立 Neon 測試 Branch，使用隔離的 `POSTGRES_URL`；`.env`、`.env.*` 等私密設定不得提交。API 的 `config.Load()` 預設載入 `.env`，若使用 `.env.auction-test`，必須先在啟動程序中正確載入環境變數並核對 Branch 對應 Host。

## 09. 下一次接手：從這裡繼續

### 目前停點（2026-11-17）

**SSE EventHub 與 Handler 已建立，下一步是把它接到 Auction Routes，然後發布實際的競標與結算事件。** 不要重新實作 `PlaceBid()`、`SettleAuction()` 或 Auction Worker。

| 優先 | 下一項工作 | 驗收標準 |
| --- | --- | --- |
| P0 | `main.go` 初始化共用 `AuctionEventHub` | Routes 與 Worker 使用**同一個 Hub** |
| P0 | 調整 `app.RegisterRoutes` 與 `RegisterAuctionRoutes` | 成功註冊 `GET /auctions/:id/stream` |
| P0 | `PlaceBid` Commit 成功後發布 `auction.bid.updated` | 交易失敗不發事件、成功後訂閱者即時收到 |
| P0 | Worker 結算成功後發布 `auction.completed`／`auction.expired` | SSE 能通知成交或流標 |
| P1 | React `EventSource` 訂閱並重抓詳情／出價紀錄 | UI 不須手動重新整理，重連後資料仍正確 |
| P1 | 詳情頁接上 `onPlaceBid` | 玩家可透過網頁正常競標 |
| P1 | SSE 自動清理、多人訂閱、重連測試 | 無不必要的 Subscriber 殘留 |
| P2 | 併發／Rollback／關閉期間交易測試 | 驗證競標與結算安全性 |
| P2 | 多實例事件廣播設計 | 若水平擴充，評估 PostgreSQL NOTIFY／Redis／Outbox |

### 建議的 SSE 事件路徑

```text
POST /auctions/:id/bids
  → repository.PlaceBid() 完成 Commit
  → AuctionEventHub.Publish(auction.bid.updated)
  → GET /auctions/:id/stream
  → React EventSource 接收通知
  → GET /auctions/:id + GET /auctions/:id/bids 取得最新快照

Auction Worker
  → repository.SettleAuction() 完成 Commit
  → Publish(auction.completed / auction.expired)
  → React 更新頁面
```

先以單一 Go 實例的 in-memory Hub 完成 MVP；事件可能因緩衝區滿或斷線遺失，因此不可取代資料庫快照或交易確認。

## 10. 後續規劃

- **Reliability**：併發競標壓力測試、故障注入、確保財務流水可追溯。
- **Realtime**：拍賣 SSE 完整串接、重連同步與部署後的長連線管理。
- **Deployment**：環境分離、CI/CD、容器化與正式環境安全設定。
- **NEXELYTH Unity VR**：未來以統一後端管理玩家身分、物品所有權與跨平台領取流程；目前仍為規劃。

詳見：[Frontend README](client/README.md) · [Backend README](server/README.md) · [Development Roadmap](docs/ROADMAP.md) · [Cross-Platform Architecture](docs/architecture/001-cross-platform.md)

---

**NEXELYTH Marketplace** — *Building a fantasy trading ecosystem across web and virtual worlds.*
