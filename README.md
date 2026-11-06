
# NEXELYTH Marketplace

### 虛擬資產競拍與交易平台

**React · TypeScript · Go · Gin · PostgreSQL · JWT · WebSocket · SSE**

NEXELYTH Marketplace 是一套以奇幻世界為背景的虛擬資產交易平台，使用 **React + TypeScript** 建立 Web Client，並以 **Go + Gin** 開發 REST API，透過 **Neon PostgreSQL** 管理玩家身分、背包物品、虛擬貨幣錢包與競拍紀錄。

專案核心著重於：

- **資料一致性（Data Consistency）**：透過 PostgreSQL Transaction 確保多項資料變更的原子性。
- **併發控制（Concurrency Control）**：運用 Row-Level Locking 保護競價及資產操作。
- **資產所有權（Asset Ownership）**：由後端驗證玩家是否擁有物品及操作權限。
- **競標資金管理（Bid Fund Management）**：建立出價資金保留、前筆資金釋放及交易流水機制。
- **跨平台整合（Cross-Platform Integration）**：規劃與 NEXELYTH Unity VR 共用玩家身分及虛擬資產。

目前已完成主要後端架構、認證、物品上架、競標 API 及資金保留機制，並開始進行實際交易情境驗證。

拍賣結算、背景排程、成交後物品轉移及 Unity VR 資產同步，則為後續開發重點。

---

## 01. 專案概述

NEXELYTH Marketplace 以 Go Backend 作為核心，透過 REST API 向 React Web Client 提供玩家身分、背包、錢包及競拍資料。

前端採用奇幻風格介面，包含玩家資產中心、競拍刊登頁面，以及透過 CSS 3D Transform 實作的旋轉木馬競拍殿堂。

在後端設計上，除了基本 CRUD，亦針對多人交易系統常見的資料一致性問題進行處理，例如：

- 同一件物品不得重複刊登。
- 玩家不得競標自己的商品。
- 出價必須符合最低加價限制。
- 已截止的拍賣不得接受新出價。
- 出價前必須驗證錢包餘額。
- 有效最高出價必須有對應的資金保留紀錄。
- 競價、資金扣除及交易紀錄須在同一個 Transaction 內完成。

### 功能與開發進度

| 功能模組 | 實作內容 | 狀態 |
|---|---|---|
| Authentication | 使用者註冊、登入、JWT、HttpOnly Cookie | 已實作 |
| Marketplace Inventory | 玩家背包、物品定義、所有權與狀態管理 | 已實作 |
| Auction Listing | 拍賣刊登、商品查詢、詳細資料 | 已實作 |
| Ownership Validation | 驗證物品所有權及可交易狀態 | 已實作 |
| Duplicate Listing Protection | 防止同一件物品重複有效上架 | 已實作 |
| Auction Bidding | 競標出價、最低加價及截止時間驗證 | 已實作 |
| Wallet | 錢包餘額查詢及交易流水資料模型 | 已實作 |
| Bid Fund Hold | 出價資金保留、扣款及流水紀錄 | 已實作並完成首次競標驗證 |
| Bid Release | 前任最高出價者退款及自行提高出價 | 邏輯已實作，待完整測試 |
| Auction Settlement | 成交款項結算與賣家收款 | 待開發 |
| Asset Ownership Transfer | 得標後物品所有權轉移 | 待開發 |
| Background Worker | 自動檢查已截止拍賣並補結算 | 待開發 |
| Unity VR Integration | 共用玩家身分及跨平台資產同步 | 規劃中 |

> 已實作代表程式碼已有對應功能，不代表所有整合測試、併發測試或正式環境驗收均已完成。

---

## 02. 技術架構

### Technology Stack

| Layer | Technologies |
|---|---|
| Frontend | React 19、TypeScript、JavaScript、Vite 7 |
| UI / Styling | Tailwind CSS 4、Lucide React、Framer Motion |
| Routing | TanStack Router |
| State / Data | React Context、TanStack Query |
| Backend | Go、Gin Framework |
| Database | PostgreSQL、Neon |
| Database Driver | pgx/v5、pgxpool |
| Authentication | JWT、bcrypt、HttpOnly Cookie |
| Realtime | WebSocket、Server-Sent Events |
| Database Migration | golang-migrate |
| Development Tools | Air、ESLint、Git、GitHub |

前端目前同時使用 JavaScript 與 TypeScript，並持續進行漸進式 TypeScript 整理。

WebSocket 與 SSE 已具有基礎實作，但完整競標即時推播仍在後續整合階段。

### 系統架構

```text
                 NEXELYTH Marketplace
                     React Web
                  TypeScript / Vite
                           |
                           | REST API / JSON
                           | JWT HttpOnly Cookie
                           |
                           v
                      Go Backend
                     Gin Framework
                           |
         +-----------------+-----------------+
         |                 |                 |
         v                 v                 v
   Authentication       Inventory         Auction
         |                 |                 |
         |                 |                 |
         +-----------------+-----------------+
                           |
                           v
                     Wallet System
                           |
                           v
                   Repository Layer
                           |
                       pgx / pgxpool
                           |
                           v
                    Neon PostgreSQL
```

目前 Web Client 透過 Go REST API 存取資料，不會直接連線 PostgreSQL。

後端主要負責：

1. 使用者身分與操作權限驗證。
2. 資產所有權及物品狀態檢查。
3. 拍賣競價規則驗證。
4. 錢包資金保留與釋放。
5. 透過 Transaction 維持跨資料表一致性。

---

## 03. 身分驗證與玩家資產管理

### Authentication

採用 JWT 與 HttpOnly Cookie 建立使用者登入流程。

主要功能包括：

- Email / Password 註冊及登入。
- bcrypt 密碼雜湊驗證。
- JWT 使用者身分識別。
- HttpOnly Cookie 保存登入憑證。
- `/auth/me` 查詢目前登入使用者。
- PostgreSQL UUID 作為使用者識別。

後端會根據經驗證的身分取得玩家 ID，不直接信任前端提交的所有權資訊。

### Marketplace Inventory

玩家背包使用兩層資料模型：

**item_definitions**

管理物品種類及基礎資料，例如：

- 物品名稱
- 分類
- 圖片
- 描述
- 是否允許交易

**inventory_items**

管理玩家實際持有的物品，包括：

- 物品 UUID
- 所有者 UUID
- 對應物品定義
- 目前物品狀態
- 建立及更新時間

透過區分「物品定義」與「實際物品」，支援多個玩家擁有同類型、但具有不同物品 ID 的資產。

---

## 04. 拍賣刊登與資料一致性

### Transactional Auction Listing

玩家建立拍賣時，後端會在同一個 PostgreSQL Transaction 中：

1. 使用 `SELECT ... FOR UPDATE` 鎖定指定背包物品。
2. 確認玩家為物品持有人。
3. 驗證物品狀態為 `AVAILABLE`。
4. 驗證物品允許交易。
5. 建立 `auction_listings` 紀錄。
6. 將 `inventory_items.status` 更新為 `LISTED`。
7. 所有步驟成功後才 Commit。

任何操作失敗均會 Rollback，避免出現商品已上架但背包狀態沒有更新的情況。

### 防止重複刊登

Migration `000014` 將 `auction_listings` 與 `inventory_items` 建立資料庫關聯。

並使用 PostgreSQL Partial Unique Index，防止同一個物品 ID 同時存在多筆有效刊登。

此設計同時利用：

- Application-Level Validation
- Row-Level Locking
- Database Constraints
- Atomic Transaction

降低單純依賴前端驗證所造成的資料競爭風險。

---

## 05. 競標系統與資金保留

### Auction Bidding

後端 `PlaceBid()` 已整合拍賣出價與錢包資金管理。

執行出價時，系統會：

1. 鎖定拍賣紀錄。
2. 驗證拍賣狀態與截止時間。
3. 禁止賣家競標自己的商品。
4. 檢查最低加價金額。
5. 查詢目前最高出價及有效保留資金。
6. 鎖定相關玩家錢包。
7. 驗證新出價者可用餘額。
8. 釋放前任最高出價者的保留資金（如適用）。
9. 扣除新出價者的可用餘額。
10. 建立出價紀錄與資金保留紀錄。
11. 寫入錢包交易流水。
12. 更新拍賣最高出價及出價者。
13. Commit Transaction。

所有資料變更必須在同一個 Transaction 中成功完成。

### Bid Fund Hold

Migration `000015_add_auction_fund_holds` 新增：

`auction_fund_holds`

| 欄位 | 用途 |
|---|---|
| id | 資金保留紀錄 UUID |
| listing_id | 對應拍賣 |
| bid_id | 對應出價 |
| bidder_id | 出價玩家 |
| amount | 保留金額 |
| status | 資金保留狀態 |
| created_at | 建立時間 |
| updated_at | 更新時間 |

保留資金具有三種狀態：

| Status | 說明 |
|---|---|
| HELD | 資金已保留 |
| RELEASED | 資金已釋放 |
| SETTLED | 資金已完成結算 |

資料庫透過 Partial Unique Index 限制同一場拍賣最多只能存在一筆 `HELD` 紀錄。

### Wallet Transactions

錢包交易流水新增兩種交易類型：

| Transaction Type | 說明 |
|---|---|
| BID_HOLD | 競標資金保留 |
| BID_RELEASE | 原有資金保留釋放 |

目前競標資金保留機制已實作。

`SETTLED` 狀態及賣家收款仍需由後續的 Auction Settlement 流程完成。

---

## 06. 實際整合測試

已使用兩組測試帳號，透過 Go REST API 與 Neon PostgreSQL 驗證競標流程。

### 測試案例：蒼藍騎士劍

**拍賣設定**

| 項目 | 測試資料 |
|---|---|
| 商品 | 蒼藍騎士劍 |
| 起標價格 | 1,000 G |
| 最小加價 | 100 G |
| 買家出價 | 1,100 G |
| 買家初始餘額 | 1,000,000 G |

### 執行結果

| 驗證項目 | 實際結果 |
|---|---|
| Auction Current Price | 1,100 G |
| Auction Highest Bidder | 已更新為測試買家 |
| Auction Bid | 已新增 1,100 G 出價 |
| Fund Hold Amount | 1,100 G |
| Fund Hold Status | HELD |
| Wallet Balance | 998,900 G |
| Wallet Transaction | BID_HOLD |
| Transaction Amount Delta | -1,100 G |

測試確認：

- 成功建立競標紀錄。
- 最高出價同步更新。
- 錢包可用餘額正確扣除。
- 資金保留紀錄與出價金額一致。
- 錢包交易流水正確寫入。
- 不符合最低加價要求的出價會被拒絕。
- 拍賣截止後的新出價會被拒絕。

### 目前尚未完成的部分

測試拍賣截止後，仍保留以下資料狀態：

```text
auction_listings.status  = active
auction_fund_holds.status = HELD
inventory_items.status    = LISTED
inventory_items.owner_id  = Seller
```

這表示目前已完成競價及資金保留階段，但尚未實作正式的拍賣成交結算。

因此，買家尚未收到物品，賣家也尚未取得成交款項。

---

## 07. 資料庫與 Migration 管理

本專案採用 Neon PostgreSQL，並使用 `golang-migrate` 管理資料庫 Schema。

目前 Migration 已建立至 Version 15。

| Version | 重點 |
|---|---|
| 000009 | Auction Listings |
| 000010 | Auction Bids |
| 000011 | Auction Category |
| 000012 | Wallet System |
| 000014 | Auction Inventory Linking |
| 000015 | Auction Fund Holds |

截至 2026 年 11 月，開發資料庫已確認：

```text
schema_migrations.version = 15
schema_migrations.dirty   = false
```

後端使用 `pgxpool` 管理 PostgreSQL 連線，並透過環境變數載入資料庫設定。

另提供 PowerShell Migration Script，支援資料庫版本查詢及 Migration 執行。

---

## 08. 後續開發規劃

### Phase 1 — Auction Settlement

**目標：完成拍賣成交後的完整交易結算。**

預計實作 `SettleAuction()`：

- 驗證拍賣已截止。
- 驗證最高出價與保留金額一致。
- 使用 Transaction 鎖定相關資料。
- 將成交款項轉入賣家錢包。
- 建立賣家收入流水。
- 更新 Fund Hold 為 `SETTLED`。
- 將物品所有權轉移至得標玩家。
- 更新拍賣成交狀態。
- 處理無人出價及流標情況。
- 確保重複結算不會造成重複付款或資產交付。

### Phase 2 — Background Worker

**目標：讓拍賣即使沒有玩家在線，也能自動結算。**

規劃於 Go Backend 中加入背景工作程序：

```text
Go Server Startup
        |
        v
Check Expired Auctions
        |
        v
Find Unsettled Listings
        |
        v
SettleAuction()
        |
        v
Commit / Rollback
        |
        v
Wait for Next Schedule
```

預計包含：

- 定期掃描已截止拍賣。
- 服務重新啟動時補處理。
- 交易失敗後重試。
- Idempotency（冪等性）保護。
- 錯誤紀錄與執行狀態追蹤。

初期將以 Go Background Worker 實作。

後續依部署需求評估獨立排程服務或常駐 Worker。

### Phase 3 — Frontend Integration

進一步完成：

- 出價頁面與錢包狀態整合。
- 出價歷史顯示。
- 商品成交狀態顯示。
- 結算結果與背包資料更新。
- WebSocket / SSE 即時通知。
- 競拍流程端對端測試。

### Phase 4 — Cross-Platform Integration

與 [NEXELYTH Unity VR](https://github.com/vc428803/NEXELYTH) 整合：

- 共用玩家身分。
- Unity 遊戲背包與 Marketplace 資產關聯。
- 遊戲背包與交易暫存區分離。
- Web Marketplace 查看待上架物品。
- 成交後將物品移至買家待領取區。
- 玩家於 Unity VR 領取物品。
- 確保 Web / VR 資產所有權一致。

### Phase 5 — Reliability & Deployment

後續依實際使用需求評估：

- Automated Integration Tests
- Concurrent Bidding Tests
- Transaction Rollback Tests
- Event-Driven Architecture
- Transactional Outbox Pattern
- Redis
- Docker
- Cloud Deployment
- CI/CD Pipeline

以上為規劃方向，並非目前均已完成的功能。

---

## 09. 跨平台架構規劃

Marketplace 未來將與 NEXELYTH Unity VR 共用玩家身分與資產管理機制。

```text
              NEXELYTH Ecosystem
                      |
          +-----------+-----------+
          |                       |
          v                       v
   Marketplace Web          NEXELYTH VR
   React / TypeScript        Unity / C#
          |                       |
          +-----------+-----------+
                      |
                      v
                  Go Backend
                      |
           +----------+----------+
           |          |          |
           v          v          v
        Identity   Inventory   Trading
           |          |          |
           +----------+----------+
                      |
                      v
                PostgreSQL
```

### 規劃中的資產交易流程

```text
          Unity VR Inventory
                   |
                   v
             Trade Storage
                   |
                   v
          Marketplace Listing
                   |
                   v
              Auction Bidding
                   |
                   v
           Auction Settlement
                   |
                   v
           Buyer Claim Storage
                   |
                   v
           Buyer VR Inventory
```

跨平台整合的核心原則是：

**所有物品所有權與交易狀態，皆由後端資料庫作為一致性來源。**

Unity VR 與 Marketplace Web 分別提供操作介面，不各自獨立決定同一件物品的所有權。

詳細規劃：

[Cross-Platform Architecture](docs/architecture/001-cross-platform.md)

[Architecture Diagram](docs/architecture/images/001-cross-platform-architecture.jpg)

---

## 10. Repository Structure

```text
nexelyth-marketplace/
│
├── client/
│   ├── public/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── config/
│   │   ├── context/
│   │   ├── hooks/
│   │   ├── pages/
│   │   ├── queries/
│   │   └── types/
│   ├── package.json
│   └── README.md
│
├── server/
│   ├── cmd/
│   │   └── api/
│   ├── internal/
│   │   ├── app/
│   │   ├── config/
│   │   ├── database/
│   │   ├── handlers/
│   │   ├── models/
│   │   └── repository/
│   ├── migrations/
│   ├── scripts/
│   │   └── migrate.ps1
│   ├── go.mod
│   └── README.md
│
├── docs/
│   ├── README.md
│   ├── ROADMAP.md
│   └── architecture/
│       ├── 001-cross-platform.md
│       └── images/
│           └── 001-cross-platform-architecture.jpg
│
└── README.md
```

前端與後端採獨立目錄，透過 REST API 交換資料。

---

## 11. 技術文件

| 文件 | 說明 |
|---|---|
| [Frontend README](client/README.md) | React 前端架構與開發環境 |
| [Backend README](server/README.md) | Go 後端、資料庫及 Migration |
| [Technical Documentation](docs/README.md) | 系統設計文件 |
| [Development Roadmap](docs/ROADMAP.md) | 跨平台整合路線圖 |
| [Cross-Platform Architecture](docs/architecture/001-cross-platform.md) | 共用玩家身分與資產交易架構 |
| [Architecture Diagram](docs/architecture/images/001-cross-platform-architecture.jpg) | 跨平台系統設計圖 |

---

## 12. 開發重點總結

NEXELYTH Marketplace 目前的主要工程成果集中在：

- Go / Gin REST API 設計與身分驗證。
- PostgreSQL 資料模型與 Migration 管理。
- 玩家背包與虛擬資產所有權驗證。
- Transactional Auction Listing。
- Row-Level Locking 與競標併發控制。
- Wallet Fund Hold 與交易流水。
- 競標資金保留的實際整合驗證。

下一階段將進一步完成成交結算與 Background Worker，建立從玩家上架、競價、資金保留到最後成交交付的完整流程。

長期目標是將 Marketplace 與 NEXELYTH Unity VR 整合，形成可共用玩家身分及數位資產的跨平台交易系統。

---

**NEXELYTH Marketplace**

*Building a fantasy trading ecosystem across web and virtual worlds.*
