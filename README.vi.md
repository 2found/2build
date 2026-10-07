# 2build

[English](README.md) | Tiếng Việt | [中文](README.zh.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

![2build — từ kế hoạch đến bàn giao đã kiểm chứng. Hình minh họa ý tưởng.](docs/assets/2build-banner.jpg)

<img src="assets/2build-mascot-transparent.png" alt="Mascot 2build: nhân vật khối cam đội mũ bảo hộ, cầm khối đã kiểm chứng." width="128" height="128" align="right">

**Xem prototype. Để Autopilot làm phần còn lại.**

**[Bắt đầu với Autopilot](#cài-đặt) · [Đọc tài liệu (tiếng Anh)](docs/install.md)**

2build giúp coding agent làm trọn một tính năng. Bạn đưa yêu cầu, xem prototype giao diện, rồi để **Autopilot** tự viết code, kiểm tra và sửa lỗi.

Với giao diện nhỏ, mục tiêu là xem prototype sau **khoảng 5 phút**; thời gian thực tế tùy tác vụ, model và dự án.

- **Chất lượng:** code được review, kiểm thử và sửa lỗi trước khi bàn giao.
- **Năng suất:** agent tự làm các bước, bạn không cần nhắc từng việc.
- **Hiệu quả:** chỉnh prototype sớm, giảm công sức làm lại khi chưa đúng ý.

Dùng với **Claude Code, Codex, Antigravity, OMP và Grok**, cùng bất kỳ coding agent nào hỗ trợ Claude Code skills.

Một sản phẩm của 2found. Tên cũ là babysit; CLI `bbs`, skill `bbs:` và trạng thái `.babysit` giữ tương thích. Xem [quy chuẩn tên gọi](BRANDING.md) (tiếng Anh).

<a id="cài-đặt"></a>

## Cài bằng một prompt

Dán prompt này vào coding agent có quyền chạy terminal:

```text
Cài 2build cho coding agent tôi đang dùng theo
https://raw.githubusercontent.com/2found/2build/main/docs/install.md. Xác định hệ điều
hành và agent hiện tại, dùng lại bbs nếu đã hoạt động hoặc cài CLI, rồi cài skill pack
chỉ cho agent này. Kiểm tra bbs --version và các skill đã cài; nếu thiếu điều kiện cần thì
báo rõ, không kết luận thành công. Cho tôi biết có cần khởi động lại không và đưa đúng
lời gọi Autopilot cho agent của tôi để chạy tác vụ nhỏ đầu tiên.
```

Bạn cần coding agent được hỗ trợ và quyền dùng model sẵn có của nó. Claude Code và Codex cần CLI trên PATH. 2build không yêu cầu tài khoản model riêng; chi phí sử dụng agent vẫn áp dụng. **Chỉ Foreman cần Orca.** Xem [cài đặt và xử lý lỗi](docs/install.md) (tiếng Anh).

<details>
<summary>Muốn tự chạy lệnh? Homebrew trên macOS hoặc Linux</summary>

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

`bbs install` tự phát hiện Claude Code, Codex và Antigravity. Với OMP, Grok và agent tương thích khác, xem [hướng dẫn cài skill](docs/install.md#omp-grok-and-other-compatible-agents). Chọn riêng bằng `bbs install claude`, `bbs install codex` hoặc `bbs install antigravity`. Khởi động lại agent sau khi cài. Không có Homebrew thì dùng [archive từ release](docs/install.md#release-archives-macos-or-linux); trên Windows, dùng WSL.

</details>

## Bắt đầu project mới

[**2build starters**](https://github.com/2found/2build-starters) cung cấp template
project có version, kèm harness cho agent, hướng dẫn kiến trúc, tests và QA cục bộ.
Template đầu tiên là `hono-bun`: API Bun/Hono với TypeScript và Zod.

`bbs bootstrap` và `bbs starter check` có trong CLI 1.95.0+. Source starter đã
public dưới dạng preview; dùng `--source` đến khi có stable release đầu tiên.
Xem README của starter để biết cách bắt đầu và các điều kiện cần thiết.

<a id="autopilot-một-ticket"></a>

## Chạy ticket đầu tiên

1. Khởi động lại agent, mở Git repo và chọn một bug hoặc tính năng nhỏ với tiêu chí kiểm tra rõ ràng. Autopilot làm việc và commit trên checkout hiện tại; tạo branch bạn muốn trước nếu cần tách công việc.
2. Gọi **skill trong chat của agent**, thay ví dụ bằng tác vụ của bạn:

   | Agent | Ví dụ |
   |-------|-------|
   | Claude Code | `/bbs:autopilot "Thêm màn hình tìm kiếm đã lưu. Cho xem prototype trước, rồi triển khai, review và QA."` |
   | Codex | `$bbs:autopilot "Thêm màn hình tìm kiếm đã lưu. Cho xem prototype trước, rồi triển khai, review và QA."` |
   | OMP | `/autopilot "Thêm màn hình tìm kiếm đã lưu. Cho xem prototype trước, rồi triển khai, review và QA."` |
   | Grok | `/bbs:autopilot "Thêm màn hình tìm kiếm đã lưu. Cho xem prototype trước, rồi triển khai, review và QA."` |
   | Antigravity / agent tương thích khác | Yêu cầu dùng skill `autopilot` đã cài cho tác vụ của bạn. |

3. Đọc kế hoạch và, với tác vụ UI, mở prototype. Chỉnh bố cục, luồng tương tác hoặc phạm vi ngay tại đây. Khi Autopilot trả block `/goal`, dán vào cùng agent để bắt đầu thực hiện. Với agent không có goal mode, thêm `--stop-after=plan` vào yêu cầu đầu tiên để dừng ở bước duyệt này, rồi làm theo lời gọi tiếp tục trong handoff.
4. Để Autopilot triển khai, review, sửa lỗi, kiểm thử và QA. Khi đạt các gate hoàn tất, nó trả commit cục bộ và bằng chứng; nếu bị chặn, báo cáo nêu phần còn thiếu. Sau khi khởi động lại, đưa ticket ID từ handoff để tiếp tục.

Ticket đầu không cần Orca, session worker mới hay cấu hình dự án. Chọn model của session trước khi chạy. [Xem tiến độ và khôi phục session](docs/companion-cli.md).

## Tiếp tục xây sản phẩm cùng 2found

Khi sản phẩm cần thêm, chọn công cụ theo bước tiếp theo:

| Bạn muốn | Khám phá |
|----------|----------|
| Thêm AI agent vào sản phẩm | [**Soot**](https://trysoot.com), powered by **2agent** — **agent as config** — thêm config vào source code. Có thêm đồng đội AI. |
| Deploy sản phẩm đã build | [**2server**](https://github.com/2found/2server) — triển khai và vận hành ứng dụng trên hạ tầng bạn sở hữu. |
| Giúp khách hàng tìm thấy sản phẩm | [**2market**](https://2found.dev/#2market) — không gian marketing đang phát triển, kết nối bối cảnh sản phẩm, nội dung và các kênh trong một quy trình. |

Bắt đầu với 2build cho công việc engineering; tìm hiểu các sản phẩm này khi nhu cầu xuất hiện.

## Chọn theo công việc

Autopilot chọn một trong năm hình mẫu của đội xây dựng sản phẩm. Bạn cũng có thể chỉ định workflow hoặc gọi skill trực tiếp.

| Hình mẫu | Công việc |
|----------|-----------|
| Prototyper | Kiểm chứng ý tưởng rủi ro trước khi đầu tư code production. |
| Builder | Hoàn thành tính năng hoặc sửa bug qua review và QA. |
| Sweeper | Loại bỏ phần dư hoặc tối ưu điểm nóng đã đo, giữ nguyên hành vi. |
| Grower | Cải thiện copy, conversion hoặc thử nghiệm tăng trưởng có đo lường. |
| Maintainer | Tìm nguyên nhân lỗi, củng cố độ tin cậy, bảo mật và dependency. |

Xem [danh mục skill](docs/skills.md) để chọn skill riêng và ví dụ workflow.

<a id="cấu-hình-foreman"></a>
<a id="foreman-dự-án-nhiều-ticket"></a>

## Dự án lớn hơn: Foreman

Với nhiều ticket phụ thuộc nhau, [Foreman](docs/foreman.md) dùng [Orca](https://www.onorca.dev) để lên kế hoạch tổng thể, tách ticket con vào worktree, giao việc đã sẵn sàng và QA toàn dự án. Mặc định bạn duyệt kế hoạch/thiết kế trước khi giao việc triển khai; `--auto` giao bước duyệt đó cho worker. Chính sách finish đã cấu hình quyết định cách bàn giao.

<details>
<summary>Chi tiết CLI và cấu hình</summary>

## Skill và CLI

Skill là các quy trình dành cho agent; `bbs` là CLI hỗ trợ chúng. `/bbs:foreman` chạy bộ điều phối dự án; `bbs foreman` quản lý tra cứu chính sách model, bản ghi Foreman bền vững, các quy ước và báo cáo. `/bbs:autopilot` chạy quy trình một ticket; `bbs autopilot` cung cấp các công cụ checkpoint và trạng thái. `bbs ticket` quản lý danh tính ticket, quan hệ DAG, bằng chứng, môi trường kiểm thử, bàn giao và dọn dẹp.

Một số lệnh CLI hữu ích:

```bash
bbs dashboard
bbs foreman report <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot snapshot --json
bbs autopilot recover --json
```

`snapshot` đọc trạng thái chuẩn của ticket và bằng chứng của các gate; `recover` bổ sung các trích đoạn tài liệu có giới hạn để tiếp tục công việc.

Chạy `bbs <subcommand> --help` để xem cách dùng. Chi tiết: [CLI hỗ trợ](docs/companion-cli.md), [profile](docs/profiles.md) và [vận hành](docs/operations.md).

</details>

## Tài liệu (tiếng Anh)

[Cài đặt và xử lý lỗi](docs/install.md) · [Chọn skill](docs/skills.md) · [Xem tiến độ](docs/companion-cli.md) · [Điều phối dự án](docs/foreman.md)

Với coding agent, [llms.txt](llms.txt) dẫn thẳng đến hướng dẫn Markdown và các quy ước runtime.

## Cấu trúc repo

- `bbs` — binary đa lệnh (kết quả build được gitignore; `go build -o bbs ./cmd/bbs`). Hook là lệnh con đã biên dịch, `bbs hooks <name>`.
- `.claude/skills/` — skill cho agent và tài liệu quy trình.
- `internal/` — CLI Go và các dịch vụ.
- `web/` — dashboard SPA, được nhúng trong bản phát hành.
- `tests/`, `docs/` — bộ kiểm chứng và tài liệu người dùng.

## Telemetry

Việc sử dụng skill được ghi cục bộ dưới dạng JSONL trong `~/.babysit/analytics/`; telemetry là kênh phản hồi chính cho các lần chạy không có người giám sát.

## Giấy phép

MIT.
