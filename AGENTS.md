# herdr-deck — Agent Development Guide

## 部署规则

每次代码改动后，**必须**使用部署脚本，**不能**手动复制文件或手动启动进程。

```bash
bash scripts/deploy-and-run.sh
```

这个脚本会按顺序完成以下操作：

1. **杀掉所有旧进程** — 匹配 `index.js.*3906`、`herdr.agentview`、`ulanziPlugin` 三种模式的进程全部杀死，等待 2 秒确认死干净
2. **等待 UlanziStudio 就绪** — 循环检测端口 3906 是否在监听，最多等 30 秒
3. **同步源码** — 将 `src/` 下所有文件复制到插件安装目录
4. **同步依赖** — 确保 `node_modules/`（含 `sharp`）存在
5. **启动插件** — 清空旧日志，后台启动，确认 PID 存活
6. **输出日志** — 显示启动日志，确认渲染正常

### 规则

- 任何时候修改了 `src/` 下的文件，运行 `bash scripts/deploy-and-run.sh`
- 不要手动 `cp` 文件到插件目录
- 不要手动 `kill` 进程后手动 `node src/index.js`
- 不要同时运行多个插件实例（会导致 keydown 事件路由到旧进程）

### 调试

```bash
tail -f /tmp/herdr-deck.log    # 实时日志
grep "input" /tmp/herdr-deck.log  # 查看按键事件
grep "nav" /tmp/herdr-deck.log    # 查看导航事件
grep "error\|Error\|fail" /tmp/herdr-deck.log # 查看错误
```
