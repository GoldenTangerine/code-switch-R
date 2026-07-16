/**
 * @name: 清理临时测试程序
 * @Descripttion: 记录删除根目录旧测试程序并补充忽略规则的变更。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 11:31:42
 * @LastEditTime: 2026-07-16 11:31:42
 * @FilePath: doc/changes/2026-07-16-1131-remove-test-executables.md
 */

# 清理临时测试程序

## 变更时间

2026-07-16 11:31:42（Asia/Shanghai）

## 涉及范围

- 项目根目录临时 Windows 测试程序
- `.gitignore`

## 变更内容

- 删除 9 个未被源码或构建流程引用的 `test_*.exe` 历史构建产物。
- 增加 `/test_*.exe` 忽略规则，避免根目录临时测试程序再次进入版本控制。

## 验证结果

- 根目录不存在 `test_*.exe`。
- Git 忽略规则可匹配新的根目录 `test_*.exe` 文件。
- `updater.exe` 与 Windows 安装依赖未受影响。
