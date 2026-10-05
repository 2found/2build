# babysit

[English](README.md) | Tiếng Việt | [中文](README.zh.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

**Giao cho nó một mục tiêu. Nó lên kế hoạch, viết code, review và kiểm chứng trong lúc bạn đi vắng.**

Babysit là bộ skill cho agent kèm CLI viết bằng Go. Dùng Autopilot cho một ticket chạy tuần tự; dùng Foreman khi dự án có nhiều ticket phụ thuộc nhau và cần worker được giám sát. Các skill theo năm hình mẫu của một đội xây dựng sản phẩm — Prototyper, Builder, Sweeper, Grower, Maintainer — được chọn theo công việc, không theo loại file. Xem [các hình mẫu](.claude/skills/references/archetypes.md).

## Cài đặt

Cài CLI, rồi để Babysit tự phát hiện và cài cho các harness trên máy:

```bash
brew install lohi-ai/babysit/bbs
bbs install
```

Khởi động lại agent sau khi cài. `bbs install` hỗ trợ Claude Code, Codex và Antigravity; Claude Code và Codex cần CLI trên PATH. Muốn chọn riêng, chạy `bbs install claude`, `bbs install codex` hoặc `bbs install antigravity`. Xem [hướng dẫn cài đặt](docs/install.md), bao gồm các gói cho Linux.

Để phát triển chính Babysit, clone repo rồi chạy `go run ./cmd/bbs setup --full`; lệnh này build `bbs` và in lệnh đăng ký plugin từ checkout. `bbs update` cập nhật CLI và các plugin đã cài.

## Cấu hình Foreman

Foreman cần [Orca](https://www.onorca.dev) đã bật orchestration. Agent của worker được chọn từ chỉ định tường minh, cấu hình đã ghim cho pha tương thích, hoặc mặc định của Orca trên máy đích. Các thiết lập agent/provider/model/effort trong YAML cũ của Babysit đã bị loại bỏ và không còn được dùng.

Babysit chọn model và effort cho worker theo độ phức tạp của công việc (`simple`, `normal` hoặc `hard`) và loại pha. Lên kế hoạch, thiết kế và review thuộc pha `critical`; triển khai, QA và bàn giao thuộc pha `normal`. Chính sách ánh xạ các tổ hợp này tới tier `flash`, `pro` hoặc `max`, mỗi tier có cấu hình model cho agent được chọn. Chỉ định riêng cho pha và route hợp lệ đã lưu để resume được ưu tiên.

Xem chính sách có hiệu lực hoặc tra một lựa chọn từ thư mục dự án:

```bash
bbs foreman model --json
bbs foreman model --agent codex --complexity normal --phase-class critical --json
```

Các lệnh tra cứu này không cần ticket hay kết nối Orca. Thêm `--dir <repo-or-worktree>` để xem chính sách của dự án khác. Ghi đè từng trường dưới `foreman.models` trong `~/.babysit/settings.json` hoặc `<repo>/.babysit/settings.json`; cấu hình repo được ưu tiên hơn cấu hình toàn cục, rồi mới tới mặc định tích hợp. Ví dụ, dùng tier `max` cho pha `normal` của công việc `hard`:

```json
{
  "foreman": {
    "models": {
      "routing": {
        "hard": { "normal": "max" }
      }
    }
  }
}
```

Xem [model routing](.claude/skills/foreman/references/model-routing.md#model-tiers) để biết cách cấu hình model/effort theo agent và cách xử lý khi resume.

## Foreman: dự án nhiều ticket

Gọi **skill Foreman** trong agent (các ví dụ là lời gọi skill, không phải lệnh CLI `bbs`):

```text
# Claude Code
/bbs:foreman "Xây dựng lại luồng xử lý yêu cầu trên web và API"
/bbs:foreman --auto "Xây dựng lại luồng xử lý yêu cầu trên web và API"

# Codex
$bbs:foreman "Xây dựng lại luồng xử lý yêu cầu trên web và API"
```

Foreman khởi tạo hoặc tiếp tục dự án cha và liên kết với Orca Run tương ứng. Worker lên kế hoạch/thiết kế tạo trước một kế hoạch tổng thể, prototype (hoặc thiết kế luồng/giao diện cho công việc không có UI), cùng manifest ticket ổn định ghi rõ phạm vi và các phụ thuộc. Mặc định, bạn duyệt các tài liệu này và ticket đề xuất trước khi tạo ticket con, worktree hay giao việc triển khai. Cờ `--auto` giao bước duyệt đó cho một worker riêng kiểm tra bằng chứng và ghi lại phê duyệt; nó không bỏ qua các lệnh giữ vì an toàn hay QA về sau.

Sau khi được duyệt, các đề xuất đã chấp nhận trở thành DAG ticket với cạnh phụ thuộc rõ ràng. Foreman chỉ giao ticket đã sẵn sàng, trong giới hạn worker/tài nguyên, và giữ một worker có quyền ghi cho mỗi worktree ticket. Mỗi ticket đi qua các pha worker riêng có phạm vi giới hạn: Plan, Implement, Review và QA. Việc chọn route dựa trên độ phức tạp cùng pha, hoặc chỉ định tường minh cho từng lần giao việc.

Foreman chờ báo cáo, câu hỏi hoặc yêu cầu hỗ trợ từ worker qua Orca; nó không dò terminal hay tạo bộ hẹn giờ thử lại. Nó kiểm tra tài liệu, phiên bản sửa đổi và verdict của từng pha trước khi đi tiếp. Khi ticket vượt qua các gate, chính sách kết thúc đã cấu hình (`review`, `land` hoặc `pr`) quyết định cách bàn giao. Foreman xác minh kết quả, giải phóng worker đã kết thúc và các lease, đóng các tab/cửa sổ Orca do nó sở hữu, và chỉ xóa worktree sạch đủ điều kiện, vẫn giữ branch.

Việc kết thúc dự án cũng do worker thực hiện: các worker kiểm tra và bàn giao/dọn dẹp được ủy quyền phải kết thúc trước, rồi worker QA cuối chỉ đọc kiểm tra đúng base đã bàn giao hoặc bản ghép QA được giữ lại. Foreman chỉ hoàn tất dự án cha sau khi bằng chứng cuối, dọn dẹp và trạng thái sẵn sàng đều đạt. Các ticket tương tác với nhau còn được QA tích hợp trước khi land; bước đó không thay thế QA cuối dự án.

## Autopilot: một ticket

Gọi **skill Autopilot** cho một ticket:

```text
# Skill Claude Code, trong session bạn đã mở
/bbs:autopilot "Thêm nút bật chế độ tối"
```

Autopilot chạy độc lập sẽ lên kế hoạch, triển khai, review và QA ngay trong session bạn khởi động. Nó không chọn model mới hay đổi model giữa chừng; hãy chọn model trước khi bắt đầu. Trong Codex, gọi skill bằng `$bbs:autopilot`. Nó hoạt động không cần Orca và không mở session worker chạy nền.

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
