# AccountingDeployPoc.Backend

正風會計師事務所｜客戶與案件管理系統（後端）

> **Project Status:** Active Development / Prototype  
> **Last Reviewed:** 2026-11-18  
> **Backend:** ASP.NET Core 9 + EF Core 9 + SQL Server  
> **Repository:** https://github.com/vc428803/AccountingDeployPoc.Backend  
> **Development Model:** Single Developer / Occasional Remote Development

> [!IMPORTANT]
> 本 README 區分「程式碼已實作」、「本機資料庫已驗證」及「公司環境尚待確認」。2026-11-18 的 Migration 驗證於**家中獨立 SQL Server Express**、本機修復分支進行，**不代表 GitHub `main` 或公司資料庫已同步套用修復**。

## 1. 專案概述

本系統供會計師事務所內部管理使用，採前後端分離架構。後端提供 RESTful API，前端採 React 與 TypeScript。

主要範圍：

- 員工與部門資料、外部 HR 同步
- 員工登入、第二階段驗證、首次修改密碼
- 群組、模組及特殊操作權限
- 客戶基本資料、多位聯絡人、接案與審核
- 合約案件、工作進度、報表與管理功能（部分待擴充）

目前仍為開發原型，存在開發用 OTP、郵件服務及資料 Seeder，不可視為已達正式上線標準。

## 2. 技術架構

| Layer | Technology |
| --- | --- |
| Frontend | React + TypeScript + Vite + Tailwind CSS |
| Backend | ASP.NET Core Web API (.NET 9) |
| ORM | Entity Framework Core 9 |
| Database | Microsoft SQL Server |
| Authentication | Server-side Session + Cookie |
| Authorization | Group-based Module Permission |
| Password Security | ASP.NET Core Identity PasswordHasher (PBKDF2) |
| API Documentation | Swagger / OpenAPI |
| Development IDE | Visual Studio 2022（支援 .NET 9） |
| Version Control | Git；現階段使用 GitHub |

```text
React Frontend
      |
      | HTTP / JSON + Session Cookie
      v
ASP.NET Core Web API
      |
      +-- Controllers
      +-- Authentication / AuthService / SessionAuthenticationHandler
      +-- Authorization / ModulePermissionHandler / PermissionService
      +-- Business Services / EmployeeSyncService / DevGroupPermissionSeeder
      +-- Entity Framework Core / AppDbContext
                        |
                        v
                    SQL Server
```

Controller 負責 HTTP 請求及回應，Service 處理可重用業務邏輯，AppDbContext 負責資料存取。部分客戶建立、編號與審核邏輯目前仍在 `CustomersController`，可於後續重構。

## 3. 後端專案結構

```text
AccountingDeployPoc.Api/
|
+-- Authorization/
|   +-- ModuleAuthorizeAttribute.cs
|   +-- ModuleAuthorizationPolicyProvider.cs
|   +-- ModulePermissionRequirement.cs
|   +-- ModulePermissionHandler.cs
+-- Controllers/
|   +-- EmployeesController.cs
|   +-- CustomersController.cs
|   +-- DevEmployeeTransferController.cs
+-- Features/
|   +-- Auth/
|   |   +-- AuthController.cs
|   |   +-- AuthService.cs
|   |   +-- SessionAuthenticationHandler.cs
|   +-- Infrastructure/Email/
|       +-- IEmailSender.cs
|       +-- DevEmailSender.cs
+-- Data/
|   +-- AppDbContext.cs
|   +-- GroupSeedData.cs
|   +-- ModuleSeedData.cs
+-- Models/
|   +-- Employee.cs
|   +-- Department.cs
|   +-- UserAccount.cs
|   +-- HrEmployeeStaging.cs
|   +-- Group.cs
|   +-- Module.cs
|   +-- GroupModulePermission.cs
|   +-- SpecialPermission.cs
|   +-- GroupSpecialPermission.cs
|   +-- Customer.cs
|   +-- CustomerContact.cs
+-- Services/
|   +-- EmployeeSyncService.cs
|   +-- PermissionService.cs
|   +-- DevGroupPermissionSeeder.cs
+-- DTOs/
+-- Migrations/
+-- Properties/
+-- Program.cs
+-- appsettings.json
+-- AccountingDeployPoc.Api.csproj
```

以上是主要檔案示意，並非完整清單。

| Component | Responsibility |
| --- | --- |
| `Program.cs` | DI、Session、CORS、Authentication、Authorization |
| `AppDbContext.cs` | EF Core Entity Mapping、索引、關聯 |
| `AuthService.cs` | 密碼與驗證碼、修改密碼 |
| `SessionAuthenticationHandler.cs` | 從 Session 建立 ClaimsPrincipal |
| `EmployeeSyncService.cs` | HR Staging 同步與員工帳號建立 |
| `PermissionService.cs` | 模組權限與特殊操作權限 |
| `CustomersController.cs` | 客戶查詢、新增、產號與審核 |

## 4. Authentication：登入與 Session

### 4.1 登入流程

本專案目前不使用 JWT；身分由後端 Session 保存，前端透過 Cookie 維持登入。

```text
POST /api/auth/login
    | 密碼驗證
    v
PendingEmployeeNo
    |
    v
POST /api/auth/verify-code
    |
    +-- MustChangePassword = true
    |       |
    |       v
    |   VerifiedEmployeeNo
    |       |
    |       v
    |   POST /api/auth/change-password
    |       |
    |       v
    |   重新登入
    |
    +-- MustChangePassword = false
            |
            v
    AuthenticatedEmployeeNo
            |
            v
    GET /api/auth/me -> Protected API
```

| Session Key | Purpose |
| --- | --- |
| `PendingEmployeeNo` | 帳密通過，尚未完成第二階段驗證 |
| `VerifiedEmployeeNo` | 已通過第二階段驗證，等待修改初始密碼 |
| `AuthenticatedEmployeeNo` | 已完成驗證的登入身分 |

`SessionAuthenticationHandler` 會檢查帳號、員工是否存在及啟用，以及 `MustChangePassword` 狀態，並建立包含 `NameIdentifier = EmployeeNo` 的 ClaimsPrincipal。**不可將前端傳入的 EmployeeNo 直接當成已驗證身分。**

### 4.2 密碼及尚未正式化的功能

- `UserAccounts.PasswordHash` 使用 ASP.NET Core Identity `PasswordHasher`（PBKDF2）。
- HR 新增帳號的原型初始密碼為員工編號，`MustChangePassword = true`；新密碼至少 12 個字元。
- 第二階段驗證目前仍使用開發用固定碼 `123456`；`DevEmailSender` 只輸出開發 Log。
- Session 目前使用記憶體快取，IdleTimeout 為 15 分鐘。
- 正式上線前須完善隨機 OTP、有效期限、重試限制、實際郵件服務、Session 持久化、CSRF 保護及稽核。

> [!WARNING]
> 開發驗證碼與初始密碼策略不可直接用於正式環境。正式環境不得提供 Development 專用端點或 Seeder。

## 5. HR 員工資料整合

```text
External HR System
       |
       v
HrEmployeeStaging
       |
       v
EmployeeSyncService
       |
       +--> Employees
       +--> UserAccounts
```

主要資料表為 `HrEmployeeStaging`、`Departments`、`Employees`、`UserAccounts`。

`EmployeeSyncService.SyncFromStagingAsync()` 會：

1. 讀取並轉換 HR Staging 資料。
2. 檢查空白、重複員工編號；同批資料比對忽略大小寫及前後空白。
3. 以資料庫交易執行整批同步。
4. 更新既有員工，並為新員工建立員工與登入帳號。
5. 依離職日期更新 `Employee.IsActive`、`UserAccount.IsActive`。

資料庫另對 `Employees.EmployeeNo` 及 `UserAccounts.Username` 建立唯一索引。索引本身不取代 Service 層的正規化與重複檢查。

### 離職規則（Rule B）

```text
QuitDate = NULL       -> Active
QuitDate > Today      -> Active
QuitDate <= Today     -> Inactive
```

`Today` 以台灣當地日期為準。員工在離職日期**當日**起停用；不提供一般後台手動啟停。

### 開發與同步端點

| Method | Endpoint | Purpose |
| --- | --- | --- |
| POST | `/api/dev/employee-transfer` | Development 專用：寫入模擬 HR Staging |
| POST | `/api/employees/sync-from-staging` | 同步 Staging 至正式員工主檔 |

> [!NOTE]
> 既有 README 所記錄的 HR 同步端點使用 `[Authorize(Roles = "HrAdmin")]`，但 SessionAuthenticationHandler 未建立相應 Role Claim；接手時應優先檢查與修正授權策略。

## 6. Authorization：群組與模組權限

目前設計：**每名員工最多屬於一個 Group；一個 Group 可包含多名員工。**

```text
Employee -- GroupId --> Group
                          |
                          +--> GroupModulePermissions --> Module
                          |
                          +--> GroupSpecialPermissions --> SpecialPermission
```

```csharp
public enum ModuleAccessLevel
{
    None = 0,
    Read = 1,
    Write = 2
}
```

| Table | Responsibility |
| --- | --- |
| `Groups` | 群組定義 |
| `Modules` | 模組定義 |
| `GroupModulePermissions` | 群組對模組的 Read / Write |
| `SpecialPermissions` | 特殊操作定義 |
| `GroupSpecialPermissions` | 群組特殊操作授權 |
| `Employees.GroupId` | 員工群組歸屬 |

### 群組與模組

| Code | Group |
| --- | --- |
| G01 | 合約作業群組 |
| G02 | 合約審核群組 |
| G03 | 財務作業群組 |
| G04 | 管理設定群組 |
| G05 | 系統管理群組 |
| G06 | 唯讀查詢群組 |

已定義模組：`DASHBOARD`、`CUSTOMER`、`EMPLOYEE`、`CONTRACT`、`WORK_TRACKING`、`REPORT`、`SYSTEM_SETTINGS`、`GROUP_MANAGEMENT`。

### 開發用權限配置

| Group | Dev Seeder Configuration |
| --- | --- |
| G01 | DASHBOARD Read、CUSTOMER Write、CONTRACT Write |
| G02 | DASHBOARD Read、CUSTOMER Read、CONTRACT Read |
| G03 | DASHBOARD Read、CUSTOMER Read、CONTRACT Read |
| G04 | DASHBOARD Read、EMPLOYEE Read |
| G05 | DASHBOARD Read、CUSTOMER Write、CONTRACT Write、EMPLOYEE Write、GROUP_MANAGEMENT Write |
| G06 | DASHBOARD Read、CUSTOMER Read、CONTRACT Read、EMPLOYEE Read |

`DevGroupPermissionSeeder` 只新增不存在的權限紀錄，不會覆蓋既有設定。**Seeded Group / Module 不等於已完成所有業務 API，更不等於所有群組都自動取得權限。**

### 權限檢查流程

```text
HTTP Request
   -> SessionAuthenticationHandler
   -> ModuleAuthorizeAttribute
   -> ModuleAuthorizationPolicyProvider
   -> ModulePermissionHandler
   -> PermissionService
   -> SQL Server Permission Check
   -> Allowed / Denied
```

Controller 用法：

```csharp
[ModuleAuthorize("CUSTOMER", ModuleAccessLevel.Read)]
[ModuleAuthorize("CUSTOMER", ModuleAccessLevel.Write)]
```

`PermissionService` 需確認員工有效、群組已指派且啟用、模組已啟用、授權等級足夠。`HasSpecialPermissionAsync()` 已有查詢能力，但特殊操作的完整 API 授權仍待完善。

## 7. 客戶管理與接案審核

### 7.1 資料模型

- `Customers`：客戶主檔、接案會計師、審核狀態及時間。
- `CustomerContacts`：一名客戶可包含多位聯絡人。
- `Customers.AcceptingAccountantEmployeeNo` 參照 `Employees.EmployeeNo`。
- 刪除客戶時，聯絡人採 Cascade；接案會計師外鍵採 Restrict。

主要欄位包含：`CustomerNo`、`CustomerName`、`UniformNo`、`TaxRegistrationNo`、`Industry`、`OrganizationType`、`OrganizationSubType`、`RegisteredAddress`、`ContactAddress`、`ResponsiblePerson`、`TaxBureau`、`AcceptingAccountantEmployeeNo`、`ApprovalStatus`、`CreatedAt`、`ReviewedByEmployeeNo`、`ReviewedAt`。

### 7.2 年度客戶編號

```text
C + YYYY + NNNN

C20260001
C20260002
```

年度以台灣日期計算，年度流水號從 0001 起；單年度上限 9999。產號與新增使用 SQL Server Transaction，搭配 `sp_getapplock` 控制併發。`Customers.CustomerNo` 設有唯一索引；前端不可自行產生正式編號。

### 7.3 建立與審核

`POST /api/customers` 需要 CUSTOMER Write 權限，流程為：檢查請求、驗證承辦會計師、產生編號、建立客戶與聯絡人、設定 `ApprovalStatus = Pending`。

```text
Pending -> Approved
        -> Rejected
```

目前審核原則：已登入、僅能審核指派給自己的 Pending 客戶、不得重複審核、記錄審核者及 UTC 時間。使用條件式 `ExecuteUpdateAsync` 降低併發重複審核風險。退回補件、重送審仍待實作。

目前一般客戶清單主要以 CUSTOMER Read 控制，**尚未完整實作依承辦人、部門或小組的 Data Scope 過濾**。

## 8. 主要 API

| Method | Endpoint | Purpose |
| --- | --- | --- |
| POST | `/api/auth/login` | 帳密驗證 |
| POST | `/api/auth/verify-code` | 第二階段驗證 |
| POST | `/api/auth/change-password` | 首次修改密碼 |
| GET | `/api/auth/me` | 取得目前登入者 |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/employees` | 員工列表（EMPLOYEE Read） |
| GET | `/api/employees/options` | 啟用員工選項 |
| POST | `/api/employees/sync-from-staging` | HR 同步 |
| POST | `/api/dev/employee-transfer` | 開發用 HR 資料 |
| GET | `/api/customers` | 客戶清單與搜尋 |
| GET | `/api/customers/{id}` | 客戶明細 |
| POST | `/api/customers` | 新增客戶 |
| GET | `/api/customers/pending-approvals` | 本人待審核清單 |
| POST | `/api/customers/{id}/review` | 客戶審核 |
| GET | `/api/deployment-test` | API 運行測試 |

客戶審核請求示例：

```json
{ "decision": "Approved" }
```

或：

```json
{ "decision": "Rejected" }
```

客戶更新／刪除 API 不在目前確認的已完成範圍。實際 API 格式以 Controller 為準。

## 9. 本機開發與資料庫初始化

### 9.1 環境需求

- .NET 9 SDK
- Visual Studio 2022（支援 .NET 9）或相容 IDE
- Microsoft SQL Server（例如本機 SQL Server Express）
- EF Core CLI（`dotnet-ef` 9.x）
- Git（或其他能取得完整原始碼、包含 `Migrations/` 的方式）

### 9.2 取得程式碼

```powershell
git clone https://github.com/vc428803/AccountingDeployPoc.Backend.git
cd AccountingDeployPoc.Backend\AccountingDeployPoc.Api
dotnet restore
```

### 9.3 本機連線設定：User Secrets

程式使用 `ConnectionStrings:DefaultConnection`。請在 `Development` 環境設定本機 SQL Server，不要使用公司正式環境帳密。

在 API 專案目錄執行（如果 `.csproj` 尚未包含 `UserSecretsId`，先執行第一行）：

```powershell
dotnet user-secrets init
dotnet user-secrets set "ConnectionStrings:DefaultConnection" "Server=.\SQLEXPRESS;Database=AccountingSystemDev;Trusted_Connection=True;TrustServerCertificate=True;"
```

`dotnet user-secrets init` 只需要在專案尚未啟用 User Secrets 時執行；已設定者不必重複初始化。也可在 Visual Studio 專案上使用「管理使用者祕密」。

- `.csproj` 中的 `UserSecretsId` 可提交版本控制。
- `secrets.json` 的本機內容不應提交。
- `appsettings.json`、`appsettings.Development.json` 若含有帳密或私有連線資訊，不得提交。
- User Secrets 只適合開發環境，**不是正式環境的秘密管理機制**。
- 若版本庫歷史曾公開有效 SQL Server 密碼，須依公司流程輪替密碼並處理洩露風險；僅從目前檔案刪除密碼不足以撤銷外洩。

> [!CAUTION]
> 執行 `Update-Database` 之前，務必確認目標伺服器與資料庫名稱。**不要在不確定連線目標時，對公司環境直接執行 Migration。**

### 9.4 一次套用所有尚未執行的 Migration

在 API 專案目錄執行：

```powershell
dotnet ef database update
```

若尚未安裝 EF CLI：

```powershell
dotnet tool install --global dotnet-ef --version "9.*"
```

Visual Studio Package Manager Console 亦可明確指定本機 SQL Express：

```powershell
Update-Database -Connection "Server=.\SQLEXPRESS;Database=AccountingSystemDev;Trusted_Connection=True;TrustServerCertificate=True;"
```

**重要：** 一次 `Update-Database` 會依序執行所有尚未套用的 Migration；**不需要逐個執行**。若資料庫已有 14 個，而程式碼新增至 17 個，EF Core 只會執行第 15～17 個。已套用版本保存在 `__EFMigrationsHistory`。

### 9.5 確認套用歷史

在 SSMS 對指定的**本機開發資料庫**執行：

```sql
USE AccountingSystemDev;

SELECT MigrationId, ProductVersion
FROM dbo.__EFMigrationsHistory
ORDER BY MigrationId;

SELECT COUNT(*) AS AppliedMigrationCount
FROM dbo.__EFMigrationsHistory;
```

### 9.6 啟動 API

```powershell
dotnet run --launch-profile https
```

既有開發設定：

| Service | URL |
| --- | --- |
| HTTPS | `https://localhost:7055` |
| HTTP | `http://localhost:5152` |
| Swagger | `https://localhost:7055/swagger` |

啟動成功不等於資料庫連線或 API 功能已通過測試，仍需實際檢查資料存取。

### 9.7 開發權限 Seeder

完成 Migration，並確認使用的是本機 `AccountingSystemDev` 後：

```powershell
dotnet run --launch-profile https -- --seed-dev-group-permissions
```

此操作只用於 Development；不得對正式資料庫執行。

### 9.8 前端連線

原 README 記錄的開發 CORS 來源：

```text
http://localhost:5173
http://localhost:8021
```

前端跨來源請求需帶上 Session Cookie：

```typescript
fetch(url, { credentials: "include" });
```

實際部署需核對 HTTPS、Cookie、SameSite、CORS 與 CSRF 設定。

---

## 10. EF Core Migration 安全維護規範（重要）

本專案**目前未導入 CI**，採「開發前檢查、產生後檢查、本機驗證、定期空白資料庫測試」方式管理 Migration。這套流程不依賴 GitHub Actions，未來即使改用其他版控平台仍可維持。

### 10.1 核心原則

1. **原則上不得修改已在任何共用環境套用過的歷史 Migration。** 需要變更 Schema 時，建立新的 Migration。
2. 變更 Entity、Fluent API、索引或關聯前，先確認現有 Model 與 Migration 歷史。
3. 產生 Migration 後，檢查 `Up()`、`Down()`、`AppDbContextModelSnapshot`；不可只看 Build 成功。
4. 識別可能造成資料遺失的 `DropColumn`、`DropTable`、`AlterColumn`、非空欄位、外鍵與唯一索引。
5. **公司現有資料庫升級**與**全新空白資料庫初始化**是兩項不同的測試，必須分別考量。
6. 不在正式資料庫上做探索性 Migration 測試；應先備份及驗證可回復方案。
7. 不將任何公司帳密、正式資料、未遮罩連線字串提交至原始碼。

### 10.2 每次新增 Migration 的標準流程

先檢查 Git 工作狀態、目前 Migration 與資料庫套用歷史：

```powershell
git status
dotnet ef migrations list
```

> `dotnet ef migrations list` 應以正確的 Development 連線設定執行；若不能安全連線，改先檢查 `Migrations/` 目錄及目標環境設定。

在確認 Entity 與 `AppDbContext` 變更合理後，新增 Migration（名稱請依實際需求更改）：

```powershell
dotnet ef migrations add AddCustomerDataScope
```

執行前檢查：

- 新增的 `*.cs`、`*.Designer.cs`、`AppDbContextModelSnapshot.cs` 差異。
- `Up()` 的修改是否符合預期；`Down()` 的回復行為與資料損失風險。
- 新的唯一索引是否受既有重複資料影響。
- 外鍵是否存在孤兒資料、刪除行為是否合理。
- 非空欄位的既有資料填補策略。
- Migration 是否誤包含與本次需求無關的欄位操作。

**僅在確認目標為獨立開發／測試資料庫後**執行：

```powershell
dotnet ef database update
```

驗證後再 Commit **程式碼與 Migration 檔案**。不 Commit 本機資料庫、User Secrets 或正式帳密。

### 10.3 定期驗證「從零初始化」

當新增重要 Migration、修改外鍵／索引，或準備換電腦、遠端開發、交接前，應用全新**專用測試資料庫**進行一次完整初始化。

例如在本機 SQL Express，先以 SSMS 建立空白 `AccountingMigrationTest`，再執行：

```powershell
Update-Database -Connection "Server=.\SQLEXPRESS;Database=AccountingMigrationTest;Trusted_Connection=True;TrustServerCertificate=True;"
```

若全部套用成功，表示這份程式碼可以在該測試條件下從空白資料庫初始化；**並不保證已有正式資料的公司資料庫升級一定成功**。完成後可刪除專用測試資料庫，但不可刪除使用中的開發或公司資料庫。

### 10.4 跨電腦開發規範

```text
公司電腦：修改程式 -> 新增 Migration -> 本機驗證 -> Commit
                                                |
                                                v
                                           同步版本控制
                                                |
                                                v
家中電腦：取得最新程式 -> 使用家中 SQL Server -> Update-Database
```

- 不複製公司資料庫密碼，不直接共用公司資料庫。
- 每台電腦以自己的安全設定連接自己的開發資料庫。
- 取得最新程式碼後，`Update-Database` 只套用未執行的 Migration。
- 如果新電腦是空白資料庫，一個 `Update-Database` 指令即可依序套用全部 Migration。
- 回公司前要先確認 Git 分支與公司 `main` 的差異；**Git 無衝突不代表資料庫 Schema 一定相容**。
- 公司環境套用前，確認備份、`__EFMigrationsHistory` 與實際 Schema，避免未知狀態直接更新。

### 10.5 與 AI 協作時的固定檢查要求

每次涉及 Entity、DbContext 或 Migration，請先提供以下資訊給協作工具，並要求一次只執行一個步驟：

> 目前專案為 ASP.NET Core 9、EF Core 9、SQL Server，單人開發，偶爾在家中以獨立資料庫維護。請先確認目標環境、Git 分支、Migration 歷史、Entity / DbContext 差異；不可直接修改已套用的歷史 Migration。新增 Migration 後請檢查 Up、Down、Snapshot、資料遺失、索引、外鍵與現有資料相容性。必須同時考量「公司既有資料庫更新」及「新電腦空白資料庫初始化」，確認後才提供 Update-Database 指令。

---

## 11. 2026-11-18 Migration 修復與本機驗證紀錄

### 11.1 問題背景

在家中以全新 SQL Server Express 初始化資料庫時，發現歷史 Migration `20261006012654_AddUserAccounts` 被後續 Commit 覆寫，從原本應**建立 `UserAccounts`** 的動作，變成修改／刪除尚不存在資料表的欄位，導致空白資料庫初始化失敗。

透過 Git 歷史版本還原原本的 `CreateTable` 行為後，確認第五個 Migration `20261006063658_UpdateUserAccountForEmployeeLogin` 能正確接續調整欄位。

### 11.2 修復與驗證範圍

| Item | Result |
| --- | --- |
| Local branch | `fix/useraccounts-migration-history` |
| 歷史 Migration 修復 | `20261006012654_AddUserAccounts`（已在家中本機 Commit） |
| User Secrets project setup | 本機 `.csproj` 變更已 Commit |
| Local SQL Server | `localhost` / `.\SQLEXPRESS` |
| Local development database | `AccountingSystemDev` |
| EF Core Migration | **14 / 14** 均已成功套用 |
| `__EFMigrationsHistory` | 已查詢確認 14 筆，EF Core 9.0.23 |
| Backend API runtime integration | **尚未驗證** |
| Company SQL Server schema comparison | **尚未驗證** |
| Merge into GitHub `main` | **尚未執行** |

此次為了定位歷史問題才**逐個執行 14 次**；一般日常初始化只需執行**一次** `Update-Database`。

### 11.3 回公司後的必要檢查

在決定合併修復分支前：

1. 確認公司 SQL Server 的 `__EFMigrationsHistory`；不要重新套用已執行的全部歷史 Migration。
2. 比對 `UserAccounts`、`Employees` 等實際欄位、主外鍵與索引。
3. 確認公司資料庫是否有手動 Schema 變更或歷史遷移差異。
4. 評估修復分支的歷史 Migration 修改是否適合納入共用 `main`。
5. 在測試環境完成驗證後，才考慮對公司環境部署變更。

> [!WARNING]
> GitHub `main` 與公司既有資料庫並不會因為家中的 Migration 測試成功而自動修復。**此節為本機實驗紀錄，不是正式部署完成證明。**

---

## 12. 目前開發狀態

原功能基準為 2026-11-13；下表增加 2026-11-18 的本機 Migration 驗證，不代表其餘功能已完成整合測試。

| Module | Status |
| --- | --- |
| ASP.NET Core + SQL Server | 基礎架構已實作 |
| EF Core Migrations | 已建立；家中修復分支 14/14 初始化成功 |
| HR Staging / Employee Sync | 已實作；同步授權待確認 |
| Password Hashing | 已實作 |
| Login / Verify Code / Session | 開發版已實作；端到端驗證待完成 |
| Group / Module Schema | 已實作 |
| Module Read / Write | 已實作 |
| Special Permission Schema | 已實作 |
| Customer Create / Contacts / Annual Numbering | 已實作 |
| Customer Approval | 初版已實作 |
| Customer Data Scope | 待開發 |
| Full Special Permission Enforcement | 待開發 |
| Contract Business APIs | 待擴充 |
| Work Tracking Business APIs | 待擴充 |
| Production Authentication / Session Security | 待強化 |
| CI/CD | 尚未導入；目前採手動檢查與本機驗證 |

## 13. 下一階段開發重點

### Priority 1：驗證目前功能與修復分支

- 在家中驗證 ASP.NET Core API 使用本機 SQL Server。
- 測試登入、驗證碼、首次改密碼、Session `/api/auth/me`。
- 測試 HR Sync、重複員工編號、交易與離職停用。
- 測試群組權限的 Read／Write 與拒絕存取。
- 回公司後先比對 Migration 歷史與實際 Schema，再決定是否合併修復分支。

### Priority 2：Authorization 與 Data Scope

- 確認 HR 同步端點 `HrAdmin` 角色來源。
- 補足 SpecialPermission 的實際授權與測試。
- 規劃 All／Department／Team／Own／Assigned 等資料範圍；目前屬規劃方向，非已完成需求。
- 後端必須負責資料查詢、修改與審核的範圍限制；不可只隱藏前端按鈕。

### Priority 3：客戶管理完善

- 客戶修改 API、資料變更紀錄。
- 補件與重新送審、審核歷程與稽核。
- 承辦關係維護與客戶資料範圍。

### Priority 4：合約與工作追蹤

- 合約基本資料、客戶與合約關聯。
- 承辦／覆核、年度與季度工作紀錄。
- 核准與簽署流程及對應特殊權限。

### Priority 5：正式環境準備

- 正式 OTP、郵件服務、登入限制。
- Session 持久化、CSRF 防護。
- 結構化 Log、稽核、測試與部署設定。
- 機密資訊治理與備份／回復程序。

## 14. 接手工程師必讀

1. **先閱讀 `Program.cs`、`AppDbContext.cs`、`EmployeeSyncService.cs`、`PermissionService.cs` 與 `CustomersController.cs`。**
2. 授權邏輯應沿用 `ModuleAuthorizeAttribute` / `PermissionService`，避免在不同 Controller 重做群組權限。
3. 使用者身分應來自可信任的 Session／ClaimsPrincipal，不可信任前端直接送來的 EmployeeNo。
4. HR 是員工基本資料及在職狀態來源；`GroupId` 等系統權限不得未經需求確認就被 HR 同步覆蓋。
5. Development 專用 OTP、DevEmailSender、HR 模擬端點、Seeder 不可直接用於正式環境。
6. **修改資料模型應新增 Migration，不應任意覆寫歷史 Migration。**
7. 新增 Schema 時，需同步確認 API 回應、前端資料模型、Migration 與現有資料相容性。
8. 即使不用 GitHub，仍可透過 .NET CLI、Visual Studio 與 SQL Server 重建開發環境；CI 並非系統執行的必要條件。

### 建議交接驗證順序

1. 取得完整原始碼，核對 .NET、EF Core、SQL Server 版本。
2. 設定本機資料庫連線，確認是獨立開發資料庫。
3. 檢查 `__EFMigrationsHistory`；在空白測試資料庫執行初始化。
4. 啟動 API 並檢查資料庫存取。
5. 驗證 Employees / UserAccounts / HR Staging。
6. 測試登入、OTP、首次修改密碼與 Session。
7. 檢查群組、模組、權限及 Seeder。
8. 測試客戶建立、多筆聯絡人、年度產號與接案審核。
9. 確認資料範圍與特殊權限的待辦規格。
10. 比對公司環境與交接環境的資料庫版本及設定差異。

## 15. 重要開發歷程

| Period | Milestone |
| --- | --- |
| 2026/09 | API 與登入驗證原型 |
| 2026/10 | Employees、Departments、HR Staging、同步與首次修改密碼 |
| 2026/11/03–05 | PBKDF2、離職停用、安全登入 |
| 2026/11/05–06 | Session Authentication 與 API 保護 |
| 2026/11/09–10 | Group / Module / Special Permission Schema |
| 2026/11/10–11 | Dynamic Authorization Policy 與 PermissionService |
| 2026/11/11–12 | 客戶模型、年度編號、建立與審核 |
| 2026/11/18 | 家中 SQL Express 修復分支驗證全部 14 個 Migration；公司環境待比對 |

原 README 參考 Commit：[Customer Approval Workflow](https://github.com/vc428803/AccountingDeployPoc.Backend/commit/1c9ae6735fb4b35cf8ff09fd3b77cd8eed513b8f)

## 16. 專案設計原則

1. Authentication 與 Authorization 分離。
2. 員工主檔與系統權限分離。
3. Module Permission 與 Data Scope Permission 分離。
4. 前端管理體驗，後端執行最終授權。
5. 資料一致性與可追蹤性優先。
6. 涉及同步、編號及狀態轉換須考慮 Transaction 與 Concurrency。
7. Prototype 與 Production 功能、設定必須清楚隔離。
8. Migration 可重現、可驗證、可交接；不得依賴個人電腦的隱藏手動步驟。

**目前開發重點：驗證本機 API／SQL Server 整合，確認 Migration 修復分支與公司資料庫相容性，再逐步推進 Data Scope Authorization、客戶審核及合約業務。**

---

## License / Internal Use

本專案目前為開發中的業務系統原型。正式使用、公開分享或部署前，須依公司政策確認資料保護、機密資訊、部署權限及授權事項。
