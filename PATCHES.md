# 私有补丁与维护契约

来源：gen2brain/gav1d v0.2.7，commit `0d06c9dc16a45726fec53e2f5f63cf1ce61d6da9`。
原始 COPYING、PATENTS 原样保留；仓库历史可追溯该提交。
用户于 2026-11-14 在 V05D / APP-2656 确认私有最小补丁镜像。

补丁范围：

1. module/import 机械迁至 github.com/appforges/gav1d。
2. 从 toImage 抽取共享 pictureToImage，算法不变；公开 PictureToNRGBA 只接受真实、仍存活的 8-bit I420/I444、CICP 1/13/6 Picture。参数只读借用，返回不别名输入的 opaque NRGBA；不解码、不应用容器变换或 ICC、不 Release/Flush。
3. Decoder.DisableScratchPooling 显式 true 时 frameContext 不取得 tc/fb pool 引用，不 Get/Put 缓存；false 保持原行为。picture pool 不在本选项范围，仍须调用者按生命周期排空、释放并丢弃 decoder。
4. 测试与本地双 Go 脚本；GitHub Actions 暂停，保留原工作流。

独立 module 可按固定版本获取，不使用宿主本地 replace/go.work。每次升级记录上游 SHA、补丁差异和模块摘要，复验两 Go、noasm、精确 RGB、所有权、取消与资源证据；发现新差异时另行评审。上游提供等价能力后可以评审迁回，不自动切换。

这不是完整 AVIF 安全准入：无硬 RSS/CPU 或立即内存归还保证，不提供 context 强制中断，不提高 Boundlet 默认额度或启用生产 AVIF。独立验收位于 Boundlet 的 tools/avif-admission。

本地脚本默认执行离线测试，明确不启用本机原生 avifdec；可显式设置 AVIFDEC_BIN 单独对照。当前原生 libavif 1.4.2/libaom 3.14.1/no-libyuv 对未修改上游和本镜像的 TestColor 均有相同五个失败：clap-10、thumb、yuv420-10、yuv422-12、yuv444-10。失败记录保留，不修改测试或算法掩盖；不将离线通过称为这些原生比较已通过。本期支持的 8-bit 窄子集另用固定独立 bilinear 金标准逐像素验收。
