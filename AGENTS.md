# gav1d 最小补丁镜像

- 2026-11-17 用户授权公开 appforges/gav1d，消费者按固定 Go 模块版本下载，不依赖本地 checkout/replace/go.work。此前“私有”描述为历史，后续发布仍仅针对 appforges/gav1d 的补丁预发布，不向上游发布；SDK 不自动升级或启用 10 位。可按需重新克隆维护。

- 上游固定 gen2brain/gav1d v0.2.7，提交 0d06c9dc16a45726fec53e2f5f63cf1ce61d6da9；保留 COPYING、PATENTS 和历史。
- 只维护 PictureToNRGBA 共享颜色入口、局部 DisableScratchPooling 与机械模块路径调整。不要重写 codec 或改默认行为。
- V05T仅PictureToNRGBA的R/B乘加改为显式float64 FMA再转float32，修正255×255 limited I420单像素49→50；普通Decode继续原路径/算法/SIMD。见PATCHES.md及冻结原失败/独立参考，不推广其他位深/格式或所有C构建。
- V05Y/APP-2749已获准新增独立PictureToNRGBA64及必要专用纯Go颜色路径：仅10位I420/I444/CICP1-13-6/full或limited，先独立参考后实现，精确RGB16/低位与借用/输入验证。原Decode/PictureToNRGBA/SIMD不改，SDK不自动升级或启用10位；V05X的明确float32/FMA求值顺序按现有site唯一方案，不能用候选输出当oracle。私有发布必须显式--repo appforges/gav1d，gh可能默认指向upstream；只PATCH私有预发布，不对上游发布。
- 使用 main 主干；不建开发分支或 worktree，不覆盖其他修改。双 Go 1.27.3/1.26.10 race、vet、build，另验 noasm。
- GitHub Actions 暂停，工作流以 .disabled 保留。凭证、构建输出不提交。
- API 输入只读借用，输出独立；Flush、Release、取消不代表硬 RSS/CPU 保证，不强制 GC 或修改全局 GOMAXPROCS。
- SDK 消费前先在 Boundlet 独立开发模块验证；本仓库发布不授权生产 AVIF 或上游 PR。
