# Tells E to G: formatting, leftovers, wrong reader (§22 to §29)

Part of the humanizer-vi tell catalog. How to work, register, output and
when not to act are in the skill itself.

## E. Định dạng theo luật

Templates also make clean formatting. The tell is decoration on every item.

### 22. In đậm trang trí

**Problem:** Bold with no reason, and list items that all start with a bold label
and a colon. Remove the bold; turn a labelled list into prose when the labels add
nothing.

**Before:**
> - **Giao diện:** Giao diện được cải thiện đáng kể.
> - **Hiệu năng:** Hiệu năng được nâng cao vượt trội.

**After:**
> Bản mới có giao diện mới và chạy nhanh hơn.

### 23. Tiêu đề viết hoa mọi chữ, emoji

**Problem:** Headings capitalize every syllable ("Hướng Dẫn Cài Đặt Chi Tiết"),
headings and list items carry emojis (🚀 ✨ 💡) or arrows as decoration, a rule
sits between every section, or a heading is written for effect. Vietnamese uses
sentence case: capitalize the first word and proper names only. Name what the
section holds.

**Before:**
> ## 🚀 Bí Quyết Tăng Tốc Website Đỉnh Cao

**After:**
> ## Làm website tải nhanh hơn

### 24. Ngoặc kép cong

**Problem:** Curly quotes (“...”) where the writer or format uses straight ones,
or the reverse. *Yếu nếu đứng một mình*: most editors convert quotes on their own.

## F. Đồ thừa từ chat và bản nháp

Remove outright.

### 25. Dấu vết chatbot

**Watch for:** Chắc chắn rồi!, Tuyệt vời!, Câu hỏi rất hay!, Bạn hoàn toàn đúng, Dưới
đây là..., Hy vọng bài viết này hữu ích, Chúc bạn thành công!, Nếu cần thêm thông
tin, đừng ngần ngại liên hệ, Bạn có muốn tôi...?, Mình có thể giúp gì thêm không?

**Problem:** A chatbot's greeting, praise, offer or sign-off left in text that
should stand alone. Remove the wrapper, keep the content.

**Before:**
> Câu hỏi rất hay! Dưới đây là cách đổi mật khẩu: vào Cài đặt, chọn Bảo mật. Hy
> vọng bài viết này hữu ích với bạn!

**After:**
> Đổi mật khẩu trong Cài đặt, mục Bảo mật.

### 26. Giới hạn kiến thức và đoán mò

**Watch for:** tính đến thời điểm hiện tại, theo thông tin mới nhất mà tôi có, hiện
chưa có nhiều thông tin, không được công bố rộng rãi, có lẽ đã từng, được cho là.

**Problem:** The text says where the model's knowledge ends, then fills the gap
with a guess. Say what the source does not show, or cut the sentence.

**Before:**
> Hiện chưa có nhiều thông tin về năm thành lập, nhưng có lẽ công ty ra đời vào
> những năm 1990.

**After:**
> Nguồn không ghi năm thành lập. (Or cut the sentence.)

### 27. Tiêu đề lặp lại ở câu đầu

**Problem:** A heading followed by a one-line paragraph restating it. Remove the
restatement.

**Before:**
> ## Tốc độ
>
> Tốc độ rất quan trọng.
>
> Trang chậm thì khách bỏ đi.

**After:**
> ## Tốc độ
>
> Trang chậm thì khách bỏ đi.

### 28. Viết về chính bài viết

**Watch for:** Bài viết này sẽ giúp bạn, Trong bài viết dưới đây chúng ta sẽ cùng,
Như đã đề cập ở trên, Bảng dưới đây so sánh, Nội dung được tổng hợp từ nhiều nguồn.

**Problem:** The text describes itself instead of its subject. Keep a source credit
the reader can follow and a caveat that changes what they do; cut the rest. One
such line is *yếu nếu đứng một mình*.

**Before:**
> Trong bài viết dưới đây, chúng ta sẽ cùng tìm hiểu ba cách tiết kiệm pin.

**After:**
> (Cut it and start with the first way to save battery.)

## G. Viết cho sai người đọc

### 29. Kể lại điều người đọc đã biết

**Watch for:** a short reply that restates the problem, walks through the
diagnosis and the evidence before the decision; background the other person wrote
or already agreed to; the answer in the last line.

**Problem:** In a reply the reader already has the context. Lead with the decision
and keep only what would change whether they agree. Act on this only when you can
see the conversation or the text plainly is a reply.

**Before:**
> Đúng rồi anh, cái này chỉ là workaround. Nguyên nhân là `MergeService` không cập
> nhật `pipeline_id`. Em đã kiểm tra QA thấy 123 lần merge, 6 dòng sai. Vì
> `MergeService` dùng chung nên em nghĩ nên tách ticket riêng.

**After:**
> Đúng, đây là workaround. Em tách ticket riêng cho `MergeService` và backfill, vì
> code đó dùng chung cho mọi account.
