# Frontend React — cấu trúc mới + UI/UX mới

> Thuộc plan [00-overview.md](00-overview.md). App cũ: `~/Workspace/VH/wavetrades/wavetrade-app`.

## 1. Vấn đề của app cũ cần giải quyết

| Vấn đề | Hậu quả | Cách giải quyết |
|---|---|---|
| Interceptor axios trả `error.response` thay vì reject | Lỗi trông như thành công; lỗi mạng làm crash | API client ném `ApiError` có kiểu; TanStack Query xử lý error state |
| Không có server-state layer, pairs fetch riêng ở 8 màn, ~75 atom Jotai copy dữ liệu server | Dữ liệu cũ, không invalidate chéo | TanStack Query là nơi duy nhất giữ server state; query key factory theo domain |
| Filter/sort trong atom global | Không share/bookmark được | Filter, sort, page nằm trong URL search params (validate bằng zod) |
| 16 bảng tự viết, không phân trang/sort, cuộn ngang trên mobile | Khó dùng, nhất là trên điện thoại | 1 component `DataTable` dùng TanStack Table + chế độ card trên mobile |
| DCA/EMA/Grid là 3 bản copy (TradesTable ×3, dialogs ×3…) | ~2.5k LOC trùng | Grid + EMA Settings đã bỏ (D1); chỉ còn DCA → viết 1 lần, dialog/badge chung đặt ở `shared/ui` |
| Menu 3 icon không nhãn, không active state, không tiêu đề trang | Khó định hướng | Sidebar có nhóm + nhãn, breadcrumb, page header, command palette |
| `confirm()` native ×31, chỉ 2 spinner, không có empty/error state | UX thô, dễ bấm nhầm lệnh tiền thật | ConfirmDialog (nhập lại tên pair cho thao tác nguy hiểm), skeleton, empty/error state chuẩn |
| Base URL hard-code, không login flow, 401 → route không tồn tại | | Cùng origin với Go; trang `/login`; route guard |
| dayjs + moment + moment-timezone; `any` ×53; exhaustive-deps tắt; không test; `dist/` commit | | dayjs duy nhất; TS strict + cấm `any`; bật đủ lint; vitest + Playwright; build không commit |

## 2. Stack

| Mảng | Chọn | Ghi chú |
|---|---|---|
| Core | React 19, TypeScript strict, Vite | |
| Router | **TanStack Router** (file-based, search params có kiểu) | Hợp với việc đưa filter vào URL; thay thế: React Router v7 |
| Server state | TanStack Query v5 | + SSE từ `/api/v1/events` để invalidate |
| Client state | Zustand (rất ít: theme, sidebar, preferences) | Bỏ Jotai |
| UI | **shadcn/ui + Radix + Tailwind CSS v4** (D4 — đã chốt, không dùng MUI) | Dark mode tốt, bảng dày đặc dữ liệu đẹp, tự sở hữu component. Date picker: shadcn Calendar (react-day-picker). Không cài `@mui/*`, Emotion, styled-components |
| Bảng | TanStack Table v8 + virtualization (`@tanstack/react-virtual`) khi > 200 dòng | |
| Form | react-hook-form + zod (`@hookform/resolvers/zod`) | Schema zod dùng chung với validate search params |
| API types | `openapi-typescript` + `openapi-fetch` sinh từ `<root>/api/openapi.yaml` | Không viết tay interface model nữa |
| Chart | `lightweight-charts` (TradingView) cho giá/nến + EMA overlay; Recharts cho equity/thống kê | |
| Toast | sonner | |
| Command palette | cmdk | |
| Ngày giờ | dayjs (+ utc, timezone) duy nhất | |
| Số / tiền | `Intl.NumberFormat` qua helper `formatMoney/formatPct/formatQty` | Nhận string decimal từ API; tính toán FE (nếu có) dùng `decimal.js-light` |
| Test | Vitest + Testing Library + MSW (mock API theo OpenAPI); Playwright smoke | |
| Lint | ESLint flat: typescript-eslint type-aware, react-hooks (bật exhaustive-deps), jsx-a11y; Prettier | |

## 3. Cấu trúc thư mục (feature-based)

```
frontend/
  src/
    app/                      # bootstrap: providers (QueryClient, Router, Theme), error boundary, SSE listener
    routes/                   # TanStack Router file routes — mỏng, chỉ ghép feature
      __root.tsx  login.tsx  _app.tsx (layout có auth guard)
      _app/index.tsx                       # Dashboard tổng
      _app/ema1d/{index,pairs,todos,trades,equity,config}.tsx
      _app/dca/{index,new,$id,config}.tsx
      _app/admin/{pairs,jobs}.tsx
    features/
      auth/        { api.ts, hooks.ts, components/ }
      ema1d/       { api.ts (queries + mutations), keys.ts, components/, hooks/, schemas.ts }
      dca/         { api.ts, keys.ts, components/ (SettingList, SettingDetail, TradesTable, dialogs), schemas.ts }
      pairs/       (danh sách pair, tạo pair, drawer chi tiết + mô tả coin markdown)
      account/     (balances, borrow/repay/transfer)
      jobs/
      # cấu hình nằm trong feature tương ứng: ema1d/components/ConfigForm, dca/components/ConfigForm
    shared/
      api/        { client.ts (openapi-fetch + ApiError), schema.d.ts (generated), sse.ts }
      ui/         # shadcn components + wrapper riêng: DataTable, PageHeader, StatCard, Money, Pct,
                  # PnL, StatusBadge, ConfirmDialog, EmptyState, ErrorState, Skeletons, FormField*
      lib/        { format.ts, date.ts, decimal.ts, url-state.ts }
      hooks/      { useMediaQuery, useDisclosure, ... }
    styles/       # tokens: màu P&L, trạng thái, spacing, typography
```

Quy tắc:

- Route chỉ import từ `features/*` và `shared/*`. Feature không import feature khác (trừ qua `shared`) — kiểm bằng `eslint-plugin-boundaries`.
- Mỗi feature có `keys.ts` (query key factory) và `api.ts` export hook `useXxxQuery` / `useXxxMutation`; mutation tự `invalidateQueries` đúng key.
- Code-split theo route (TanStack Router lazy) — bỏ `chunkSizeWarningLimit: 3000`.

## 4. Thông tin kiến trúc (IA) & điều hướng

```
Sidebar (thu gọn được; trên mobile thành drawer + bottom bar 4 mục chính)
├─ Tổng quan                 /            (KPI: equity, PnL ngày/tháng, margin level, todo lỗi, job lỗi)
├─ EMA1D
│  ├─ Bảng điều khiển        /ema1d
│  ├─ Rổ coin                /ema1d/pairs
│  ├─ Todo / hàng đợi        /ema1d/todos
│  ├─ Lịch sử lệnh           /ema1d/trades
│  ├─ Equity                 /ema1d/equity
│  └─ Cấu hình               /ema1d/config     (budget, % vốn CORE/SENSITIVE × LONG/SHORT, enabled, pause_open, max_entry_age_days, ngưỡng margin…)
├─ DCA (P3)
│  ├─ Danh sách              /dca
│  └─ Cấu hình               /dca/config       (giới hạn vốn…)
└─ Quản trị
   ├─ Pairs                  /admin/pairs      (bảng pair; bấm 1 dòng mở drawer: thông tin pair + mô tả coin markdown, sửa bằng editor + preview)
   └─ Jobs                   /admin/jobs       (trạng thái, lần chạy cuối, chạy tay)
```

Top bar gọn: tên trang + breadcrumb, ô tìm kiếm / `⌘K` (nhảy tới pair, màn hình, hành động), chỉ báo trạng thái (●) cho: kết nối SSE, job lỗi, `TRADING_ENABLED`, `pause_open`. Số dư tài khoản chuyển vào popover thay vì chiếm chỗ mọi trang.

Bảng ánh xạ route cũ → mới:

| Cũ | Mới |
|---|---|
| `/ema1d` | `/ema1d` (+ tách tab pairs/todos/trades/equity thành route con) |
| `/` (trước là EMA Settings) | `/` = Tổng quan |
| `/dca-settings*` | `/dca*` |
| `/app-settings` | Tách: phần EMA1D → `/ema1d/config`, phần DCA → `/dca/config`; phần EMA legacy bỏ |
| `/pairs`, `/coins` | `/admin/pairs` (Coins gộp vào drawer của pair) |
| `/ema-settings*`, `/grid-settings*`, `/tags`, markets (trong PairsManagement cũ), `/position-sizing`, `/avg-daily-price-change`, `/avg-monthly-price-change`, `/ema-trades`, `/ema-trackings`, `/ema-scores`, `/ema-statistics`, `/ema-trade-histories` | **Bỏ** (D1) |

## 5. Nguyên tắc UI/UX

1. **Dark mode mặc định**, có light; token màu thống nhất: lời `--pnl-up`, lỗ `--pnl-down`, trung tính; LONG/SHORT có màu + nhãn chữ (không chỉ dựa vào màu).
2. **Số liệu**: font tabular-nums, căn phải, đơn vị rõ (USDT, %, coin), làm tròn theo step size của pair; hover hiện giá trị đầy đủ.
3. **DataTable chuẩn**: sort cột, filter, phân trang/virtual, sticky header, chọn cột hiển thị, mật độ (compact/comfortable), export CSV; trạng thái trong URL; trên mobile tự đổi sang danh sách card với các cột ưu tiên.
4. **Thao tác tiền thật an toàn**: force open/close, close setting, borrow/repay → ConfirmDialog hiển thị tóm tắt tác động (pair, side, qty ước tính, giá hiện tại); thao tác nguy hiểm yêu cầu gõ tên pair. Nút disable + spinner khi đang gửi; không gửi lặp.
5. **Trạng thái đầy đủ**: skeleton khi tải, empty state có hướng dẫn hành động, error state có nút thử lại + message từ `error.code`.
6. **Thời gian thực**: SSE cập nhật todo/job/giá → badge "vừa cập nhật"; không còn nút refresh rải rác.
7. **Form**: dialog/sheet nhỏ, chia bước khi dài, validate zod inline, hiển thị giá hiện tại + % chênh lệch ngay cạnh ô nhập giá (vd `open_price`/`close_price` của DCA).
8. **Truy cập**: điều khiển bàn phím đầy đủ (Radix), focus ring, contrast AA.
9. **Mobile**: xem dashboard, todo, vị thế và bấm retry/cancel được trên điện thoại.

## 6. Màn hình chính — EMA1D

| Khu vực | Nội dung |
|---|---|
| Header KPI | Equity, realized/unrealized PnL, margin level (màu theo ngưỡng), long/short notional vs cap, trạng thái `EMA1D_ENABLED`/`pause_open` (toggle có confirm) |
| Cảnh báo | Todo FAILED, sync ngày hôm nay chưa chạy, lệch giữa desired state và vị thế |
| Rổ coin | Bảng: pair, group, status (ACTIVE/EXIT_ONLY/DISABLED), tín hiệu hiện tại, vị thế, PnL, exit rule, nút check pair; click mở drawer chi tiết có chart nến 1D + EMA14/Smooth10 (lightweight-charts) |
| Hàng đợi | Todo theo trạng thái, attempts, run_after, last_error; retry/cancel; tạo todo tay |
| Lịch sử | Lệnh đã đóng, lọc theo pair/side/khoảng ngày, tổng PnL, fee, interest, trượt giá (`ref_price` vs fill) |
| Equity | Chart equity + margin level theo ngày từ snapshots |
| Daily check | Báo cáo ngày (markdown), trạng thái PENDING/SYNCED/SENT/FAILED, chạy lại |
| Cấu hình | Form chia nhóm (Vốn & sizing, Vào/thoát lệnh, Cảnh báo, Lịch chạy), mỗi field có mô tả + giá trị mặc định; trước khi lưu hiện diff cũ → mới và tác động ước tính (vd size LONG CORE đổi từ X → Y USDT); `enabled`/`pause_open` dùng toggle có confirm ngay ở header KPI; tab lịch sử thay đổi |

## 7. Data layer — mẫu chuẩn

```ts
// shared/api/client.ts
export const api = createClient<paths>({ baseUrl: '/api/v1', credentials: 'include' })
// middleware: !res.ok → throw new ApiError(status, body.error.code, body.error.message, body.error.details)
// 401 → queryClient.clear(); router.navigate({ to: '/login' })

// features/ema1d/keys.ts
export const ema1dKeys = {
  all: ['ema1d'] as const,
  overview: () => [...ema1dKeys.all, 'overview'] as const,
  todos: (f: TodoFilter) => [...ema1dKeys.all, 'todos', f] as const,
}
// SSE 'todo.updated' → queryClient.invalidateQueries({ queryKey: ema1dKeys.all })
```

## 8. Các bước

| Bước | Nội dung | Phụ thuộc BE |
|---|---|---|
| F0 (~10 ngày) | Scaffold Vite + TS strict + lint; Tailwind v4 + `shadcn init` + theme tokens (light/dark, màu P&L, trạng thái); shell (sidebar, top bar, breadcrumb, ⌘K); router + auth guard + `/login`; API client sinh từ OpenAPI; MSW; DataTable, ConfirmDialog, Money/PnL, states; Storybook (tuỳ chọn) | P0 (spec skeleton) |
| F1 | Admin: Pairs (bảng + tạo pair chỉ cần nhập tên + drawer chi tiết có mô tả coin: `react-markdown` để xem, textarea + preview để sửa), Jobs; account popover (balances, borrow/repay/transfer) | P1 |
| F2 | EMA1D đầy đủ (mục 6, gồm màn Cấu hình) + SSE | P2 |
| F3 | DCA: list, tạo, chi tiết + trades, force open/close, màn Cấu hình | P3 |
| F4 | Playwright smoke cho luồng chính, kiểm a11y (axe), Lighthouse mobile | — |

Định nghĩa xong cho mỗi màn: có loading/empty/error, hoạt động trên viewport 375px, filter trong URL, không `any`, test cho logic không tầm thường (format, mapping trạng thái).
