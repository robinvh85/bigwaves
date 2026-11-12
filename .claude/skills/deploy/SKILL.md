---
name: deploy
description: Deploy bigwaves lên production (droplet 167.71.202.249, https://wavetrade.top:3333) bằng deploy/deploy.sh — chọn đúng commit, build trong worktree sạch, so với bản đang chạy trên server, hỏi người dùng xác nhận, chạy deploy, kiểm healthz / log / commit đang chạy, rollback khi lỗi. Dùng khi người dùng nói "deploy", "deploy lên prod", "release", "đẩy bản mới lên server", "/deploy [commit|branch]".
---

# Deploy bigwaves lên production

Cách deploy đã chốt: `deploy/deploy.sh root@167.71.202.249`. Script build trên máy dev rồi upload
binary lên server. Không đề xuất đổi sang cách khác (commit binary, GitHub Release, CI deploy…):
người dùng đã từ chối.

`deploy.sh` tự làm: `make release` (FE build + Go linux/amd64) → upload qua `tar | ssh` → backup DB
→ thay binary (giữ `.prev`) → restart → chờ `/healthz` 30 giây, không lên thì tự quay về `.prev`.
Chi tiết server xem `deploy/README.md`.

| Thứ | Giá trị |
|---|---|
| SSH | `root@167.71.202.249` |
| URL | `https://wavetrade.top:3333` (healthz: `/healthz`) |
| Binary | `/opt/bigwaves/bin/{bigwaves,bwctl}` + `.prev` |
| DB / backup | `/var/lib/bigwaves/bigwaves.db`, `/var/lib/bigwaves/backups/` |
| Env | `/etc/bigwaves/env` |
| Service | `bigwaves` (systemd), log: `journalctl -u bigwaves` |

## Không bao giờ

- Chạy `deploy.sh … --setup`: lệnh này chạy lại `setup-debian.sh` và ghi đè `/etc/caddy/Caddyfile`
  (đang sửa tay cho cổng 3333 và redirect www).
- Sửa `/etc/bigwaves/env`, đổi `TRADING_ENABLED`, bật/tắt switch strategy (ema1d / dca /
  emasettings) như một phần của deploy. Chỉ làm khi người dùng yêu cầu riêng.
- Deploy khi chưa được người dùng xác nhận ở bước 4. Một lần đồng ý chỉ áp dụng cho đúng commit
  đó, lần đó.
- Deploy binary build từ tree có thay đổi chưa commit (`vcs.modified=true`).
- Tự `git push`, merge, hay sửa lịch sử để có commit deploy.

## Các bước

### 1. Chọn commit

- Người dùng chỉ rõ commit / branch → dùng đúng cái đó.
- Không chỉ rõ → dùng `main` local (`git rev-parse main`). Nếu `main` khác `origin/main`, nói rõ
  (hơn / kém bao nhiêu commit) để người dùng biết bản deploy có trên GitHub chưa.
- Commit nằm trên branch chưa merge vào `main` → nêu ra ở bước 4, để người dùng quyết.

### 2. Build trong worktree deploy riêng

Không build trong checkout chính của người dùng (có thể đang dở việc) và không build trong worktree
đang làm feature. Dùng một worktree detached cố định:

```bash
REPO=/Users/work/Workspace/VH/bigwaves
WT=$REPO/.claude/worktrees/deploy
SHA=$(git -C $REPO rev-parse <commit>)
if [ -d $WT ]; then git -C $WT checkout --detach $SHA; else git -C $REPO worktree add --detach $WT $SHA; fi
git -C $WT status --porcelain          # phải rỗng
(cd $WT/frontend && npm ci)            # lần đầu, hoặc khi package-lock.json đổi
```

Session đang bị khoá trong worktree khác thì dùng `EnterWorktree` với `path: $WT` để chạy lệnh ở đó.

Trước khi deploy, chạy ở `$WT`:

```bash
make test
make lint
```

Đỏ thì dừng và báo người dùng, không deploy.

### 3. So với bản đang chạy

Go nhúng commit vào binary khi build (`vcs.revision`, `vcs.modified`). Server không cần `go` hay
`strings` để đọc:

```bash
ssh root@167.71.202.249 "grep -a -o 'vcs\.\(revision\|modified\|time\)=[^[:space:]]*' /opt/bigwaves/bin/bigwaves | sort -u"
```

Có `DEPLOYED_SHA` thì liệt kê những gì sắp lên:

```bash
git -C $REPO log --oneline $DEPLOYED_SHA..$SHA
git -C $REPO diff --stat $DEPLOYED_SHA..$SHA -- backend/internal/platform/db/migrations
```

- Binary cũ không có `vcs.revision` (build trước khi có skill này, hoặc build ngoài git) → ghi
  "không biết bản đang chạy", bỏ qua phần so sánh.
- `DEPLOYED_SHA` không phải tổ tiên của `$SHA` (`git merge-base --is-ancestor` lỗi) → bản mới sẽ
  **gỡ** những commit đang chạy trên server. Nêu rõ ở bước 4.
- Có file migration mới → schema sẽ đổi khi app khởi động. Rollback binary không lùi được schema,
  phải restore backup (xem "Rollback").

### 4. Hỏi xác nhận

Dùng `AskUserQuestion`, nội dung tự đứng được:

```
Deploy lên production (wavetrade.top:3333)?
  Commit:      <sha ngắn> <subject>   (main / origin/main: <trạng thái>)
  Đang chạy:   <sha ngắn | không rõ>
  Sẽ lên:      <N commit>, liệt kê subject
  Migration:   <không có | tên file>
  Test/lint:   xanh
```

Chỉ chạy tiếp khi người dùng chọn deploy.

### 5. Deploy

Chạy từ `$WT` để script build đúng tree đó. Build FE + Go mất vài phút, nên chạy nền:

```bash
cd $WT && deploy/deploy.sh root@167.71.202.249
```

(Bash `run_in_background: true`, chờ thông báo xong, không poll.)

Trước khi coi là thành công, kiểm commit trong binary vừa build:

```bash
grep -a -o 'vcs\.\(revision\|modified\)=[^[:space:]]*' $WT/backend/bin/bigwaves | sort -u
```

`vcs.revision` phải bằng `$SHA` và `vcs.modified=false`.

### 6. Kiểm sau deploy

Script in `healthy after Ns` và exit 0 là đã qua `/healthz` nội bộ. Kiểm thêm:

```bash
curl -fsS -o /dev/null -w '%{http_code}\n' https://wavetrade.top:3333/healthz     # 200
ssh root@167.71.202.249 "systemctl is-active bigwaves; \
  grep -a -o 'vcs\.revision=[^[:space:]]*' /opt/bigwaves/bin/bigwaves | sort -u; \
  journalctl -u bigwaves --since '-3min' --no-pager | grep -iE 'error|panic|fatal' | tail -20"
```

- `vcs.revision` trên server phải bằng `$SHA`.
- Log có `panic` / `fatal`, hoặc lỗi lặp lại liên tục → báo người dùng ngay kèm log, hỏi có rollback
  không (xem dưới).

### 7. Báo kết quả

Báo: commit đã deploy, commit trước đó, các commit đã lên, migration (nếu có), kết quả healthz, log
có gì bất thường không. Gợi ý người dùng tự mở `https://wavetrade.top:3333` kiểm phần vừa đổi.

## Rollback

### Script tự rollback

`/healthz` không lên trong 30 giây → script copy `bigwaves.prev` / `bwctl.prev` về và restart, rồi
exit 1. Lúc đó:

1. Báo người dùng kèm 40 dòng log script đã in.
2. Có migration trong lần deploy này → DB có thể đã sang schema mới mà binary cũ không hiểu. Hỏi
   người dùng trước khi restore backup (bước dưới).

### Người dùng muốn quay về bản trước

Hỏi xác nhận rồi mới làm:

```bash
ssh root@167.71.202.249 "cd /opt/bigwaves/bin && cp -p bigwaves.prev bigwaves && cp -p bwctl.prev bwctl && systemctl restart bigwaves && sleep 3 && curl -fsS http://127.0.0.1:8080/healthz"
```

Nếu bản vừa deploy đã chạy migration → cần restore backup làm ngay trước lần deploy đó (bản
`backups/bigwaves-*.db.gz` mới nhất có giờ trước lúc deploy). Mất dữ liệu ghi từ lúc deploy tới lúc
restore, nên **luôn hỏi người dùng trước**. Các lệnh restore nằm ở mục "Restore một bản backup"
trong `deploy/README.md`.

Cách khác (thường tốt hơn khi lỗi nhỏ): sửa lỗi, commit, rồi deploy commit mới theo đúng các bước
trên.
