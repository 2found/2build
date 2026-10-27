# TODO Backlog

## Đang mở (Open)

- [ ] Đo lại chi phí token của Foreman sau thay đổi tối ưu prompt cache (v1.108.0)
  - khi nào: sau vài project Foreman chạy bằng bản 2build mới (bbs ≥ 1.108.0, phát hành 2026-10-25 ~05:37 UTC)
  - chạy trong thư mục `2build`:
    `python3 scripts/foreman-cache-report.py --since <ngày cài bản mới>`
    (thêm `--json` để lưu kết quả và so với `docs/data/foreman-cache-baseline-2026-10-25.json`)
  - so theo **tổng chi phí mỗi project**, không theo chi phí mỗi ticket: ticket to hơn nên đắt hơn, nhưng số ticket sẽ ít đi
  - baseline và giá trị kỳ vọng: `docs/foreman-cache-economics.md` § Re-check. Các mốc chính:
    - ticket phải mở thêm phiên: 33/66 (50%) → kỳ vọng dưới 50% rõ rệt
    - số phiên trung bình mỗi ticket: 2,06 → khoảng 1,3
    - context lúc worker bắt đầu (trung vị): 58k → thấp hơn
    - lần thức do watcher bị cache lạnh: 21% → dưới 10%
    - số lượt mỗi lần thức do watcher (trung vị): 7 → thấp hơn
    - `task done` gate từ chối: 0 → lớn hơn 0 nghĩa là gate bắt được lỗi khi worker còn ấm
    - cache hit 98,4% / lãng phí do hết hạn 2,0%: giữ nguyên
  - để ý: ticket quá to (compact nhiều, plan `L` bị kẹt, review kém hơn); lạm dụng `--no-gate` (`gate: "skipped: …"` trong `$BABYSIT_HOME/tasks/*/task.json`); keep-warm tốn mà không giảm số lần thức lạnh
  - script chỉ đo phiên Claude Code; agent khác không có transcript ở `~/.claude/projects`
  - 2026-10-27: re-check done (`docs/foreman-cache-economics.md` § Re-check 2026-10-27): Foreman cold wakes fixed, worker sessions/worktree worse (2.83). v1.111.0 adds the compose check, carried gates, one-session phase QA + product review and low-effort follow-up review; measure again after a few projects on ≥ 1.111.0, also by Dispatch durations in `$BABYSIT_HOME/tasks/*/task.json` (omp projects have no transcripts)
  - added: 2026-10-25
