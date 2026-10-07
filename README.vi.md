# babysit

[English](README.md) | Tiếng Việt | [中文](README.zh.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

**Giao mục tiêu cho coding agent. Nhận lại thay đổi đã được review và kiểm thử.**

Babysit là bộ skill mã nguồn mở cho Claude Code, Codex và Antigravity, kèm CLI lưu tiến độ và bằng chứng kiểm chứng. Bắt đầu với **Autopilot**: một ticket, từ yêu cầu đến commit cục bộ, ngay trong session agent bạn đang dùng.

<a id="cài-đặt"></a>

## Cài bằng một prompt

Dán prompt này vào coding agent có quyền chạy terminal:

```text
Cài Babysit cho coding agent tôi đang dùng theo
https://raw.githubusercontent.com/lohi-ai/babysit/main/docs/install.md. Xác định hệ điều
hành và agent hiện tại, dùng lại bbs nếu đã hoạt động hoặc cài CLI, rồi cài skill pack
chỉ cho agent này. Kiểm tra bbs --version và plugin đã cài; nếu thiếu điều kiện cần thì
báo rõ, không kết luận thành công. Cho tôi biết có cần khởi động lại không và đưa đúng
lời gọi Autopilot cho agent của tôi để chạy tác vụ nhỏ đầu tiên.
```

Bạn cần coding agent được hỗ trợ và quyền dùng model sẵn có của nó. Claude Code và Codex cần CLI trên PATH. Babysit không yêu cầu tài khoản model riêng; chi phí sử dụng agent vẫn áp dụng. **Chỉ Foreman cần Orca.** Xem [cài đặt và xử lý lỗi](docs/install.md) (tiếng Anh).

<details>
<summary>Muốn tự chạy lệnh? Homebrew trên macOS hoặc Linux</summary>

```bash
brew tap lohi-ai/babysit https://github.com/lohi-ai/babysit
brew install lohi-ai/babysit/bbs
bbs install
```

`bbs install` cài cho tất cả agent được hỗ trợ mà nó phát hiện. Chọn riêng bằng `bbs install claude`, `bbs install codex` hoặc `bbs install antigravity`. Khởi động lại agent sau khi cài. Không có Homebrew thì dùng [archive từ release](docs/install.md#release-archives-macos-or-linux); trên Windows, dùng WSL.

</details>

## Babysit khác gì?

Một prompt có thể mô tả cách làm. Babysit bổ sung workflow và trạng thái trên đĩa để đi từ mục tiêu đến review và kiểm chứng, kể cả khi session phải khởi động lại.

| Bạn cần | Babysit cung cấp |
|---------|-----------------|
| Hoàn thành tác vụ mà không chỉ dẫn từng bước | Autopilot đi qua lên kế hoạch, triển khai, sửa lỗi review và QA cho một ticket. |
| Tiếp tục sau crash hoặc mất context | Yêu cầu, kế hoạch, checkpoint và handoff được lưu trên đĩa để khôi phục. |
| Biết kết quả đã được kiểm tra | Verdict review và QA được lưu; chỉ hoàn tất khi kiểm tra hiện tại đạt và không còn phát hiện nghiêm trọng chưa xử lý. |
| Chủ động bàn giao | Autopilot độc lập commit cục bộ. Bạn xem bằng chứng trước khi push hoặc mở PR. |

Dùng khi tác vụ cần bàn giao có kiểm chứng hoặc bạn muốn để agent làm việc lúc đi vắng. Sửa nhanh một chỗ có thể chỉ cần coding agent. Babysit vẫn cần môi trường kiểm thử của dự án; thiếu quyền truy cập hoặc kiểm tra bắt buộc sẽ được báo `NEEDS_CONTEXT` hoặc `BLOCKED`.

<a id="autopilot-một-ticket"></a>

## Chạy ticket đầu tiên

1. Khởi động lại agent, mở Git repo và chọn một bug hoặc tính năng nhỏ với tiêu chí kiểm tra rõ ràng. Autopilot làm việc và commit trên checkout hiện tại; tạo branch bạn muốn trước nếu cần tách công việc.
2. Gọi **skill trong chat của agent**, thay ví dụ bằng tác vụ của bạn:

   | Agent | Ví dụ |
   |-------|-------|
   | Claude Code | `/bbs:autopilot "Sửa trạng thái tìm kiếm không có kết quả và thêm regression test"` |
   | Codex | `$bbs:autopilot "Sửa trạng thái tìm kiếm không có kết quả và thêm regression test"` |
   | Antigravity | Yêu cầu dùng skill `autopilot` đã cài cho tác vụ của bạn. |

3. Khi Autopilot trả kế hoạch và block `/goal`, đọc kế hoạch rồi dán block vào cùng agent để bắt đầu build. Với agent không có goal mode, nó tiếp tục trong session hiện tại.
4. Kết quả mong đợi: commit cục bộ, bằng chứng review/QA và handoff ghi rõ thay đổi cùng kiểm tra đã chạy. Nếu bị chặn, báo cáo sẽ nêu phần còn thiếu. Sau khi khởi động lại, đưa ticket ID từ handoff cho Autopilot để tiếp tục.

Ticket đầu không cần Orca, session worker mới hay cấu hình dự án. Chọn model của session trước khi chạy. [Xem tiến độ và khôi phục session](docs/companion-cli.md).

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
