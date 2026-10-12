# Telbiz Go SDK — Agent Handoff

Repository: https://github.com/barlus-developer/golang-sdk-telbiz

ชุดนี้เป็นเอกสารสำหรับสั่ง coding agent ให้สร้าง SDK และเอกสาร repository ให้ครบ ไม่มี implementation ของ SDK รวมอยู่ในชุดนี้

## วิธีใช้งาน

1. เปิด repository เป้าหมายใน coding agent
2. นำไฟล์ในชุดนี้ไปวางที่ root ของ repository โดยตรวจสอบไฟล์เดิมก่อน ไม่เขียนทับงานที่มีอยู่โดยไม่ตรวจสอบ
3. ส่งคำสั่งภาษาอังกฤษด้านล่างให้ agent

```text
Build the Go SDK in github.com/barlus-developer/golang-sdk-telbiz.
Read AGENTS.md, BUILD_SDK.md, and the documents under docs/ before editing.
Implement every requirement and complete all delivery checklist items,
including the SDK, tests, examples, usage documentation, repository files,
and CI. Treat docs/TELBIZ_API_REFERENCE.md as the supplied provider snapshot.
New must authenticate and start automatic access-token renewal 60 seconds
before expiry. Follow docs/TOKEN_LIFECYCLE.md for concurrency, failure,
shutdown, and retry behavior. Never automatically replay SMS or top-up POSTs.
Inspect the existing repository, preserve unrelated work, and complete the
authorized local implementation. Validate and report actual results and any
remaining blockers. Do not publish releases or push changes unless asked.
```

## ไฟล์สำคัญ

| File | หน้าที่ |
|---|---|
| `BUILD_SDK.md` | ข้อกำหนดหลักและงานทั้งหมดที่ agent ต้องทำ |
| `AGENTS.md` | กฎการทำงานของ coding agent |
| `docs/TOKEN_LIFECYCLE.md` | สัญญาการต่ออายุ token และ lifecycle ของ client |
| `docs/TEST_PLAN.md` | กรณีทดสอบที่ต้องผ่าน |
| `docs/DELIVERY_CHECKLIST.md` | รายการตรวจรับงาน |
| `docs/TELBIZ_API_REFERENCE.md` | เอกสาร API ต้นทางที่ผู้ใช้ให้มา |
| `CONTRIBUTING.md` | แนวทางสำหรับ contributor |
| `SECURITY.md` | แนวทางรายงานปัญหาความปลอดภัย |
| `.github/` | Template สำหรับ issue และ pull request |

เอกสารสั่ง agent ใช้ภาษาอังกฤษ ส่วนเอกสารใช้งาน SDK ที่ agent สร้างต้องเป็นภาษาอังกฤษ มี quick start ที่นำไปรันได้จริงหลัง implementation เสร็จ

## ข้อตกลงหลัก

- Module: `github.com/barlus-developer/golang-sdk-telbiz`; Go package: `telbiz`.
- ใช้ Client ID และ Secret เดิมขอ access token ใหม่ผ่าน Auth API
- `New(ctx, Config)` ขอ token ครั้งแรกและเริ่ม background renewal โดยอัตโนมัติ
- Token อายุปกติ 3,600 วินาที: ต่ออายุประมาณนาทีที่ 59
- Caller ต้องเรียก `Close()` เมื่อเลิกใช้ client
- ครอบคลุม Auth, SMS และเติมเงินตามเอกสารที่แนบ
- จัดทำ README, API docs, ตัวอย่าง, AGENTS, CONTRIBUTING, SECURITY, CI และ release guide ให้ครบ
- ยังไม่ได้ตรวจสอบเนื้อหา repository หรือทดสอบกับ Telbiz จริงในขั้นตอนจัดทำเอกสารนี้
