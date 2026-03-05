#!/bin/bash
# BitEngine MVP — Task Runner
set -e

TASKS_DIR="$(dirname "$0")/tasks"
PROGRESS_FILE="$(dirname "$0")/.task-progress"
touch "$PROGRESS_FILE"

case "${1:-help}" in
  list)
    echo "═══ BitEngine MVP — 8 Tasks ═══"
    for i in 1 2 3 4 5 6 7 8; do
      title=$(head -1 "$TASKS_DIR/task-$i.md" | sed 's/^# //')
      if grep -q "^$i$" "$PROGRESS_FILE" 2>/dev/null; then
        echo "  ✅ $title"
      else
        echo "  ⬜ $title"
      fi
    done
    ;;
  next)
    for i in 1 2 3 4 5 6 7 8; do
      if ! grep -q "^$i$" "$PROGRESS_FILE" 2>/dev/null; then
        title=$(head -1 "$TASKS_DIR/task-$i.md" | sed 's/^# //')
        echo "下一个: $title"
        echo "执行: ./run-task.sh $i"
        exit 0
      fi
    done
    echo "🎉 全部完成！准备上 GitHub。"
    ;;
  done)
    echo "$2" >> "$PROGRESS_FILE"
    echo "✅ Task $2 marked done"
    ;;
  status)
    done=$(grep -c . "$PROGRESS_FILE" 2>/dev/null || echo 0)
    echo "进度: $done/8 tasks"
    pct=$((done * 100 / 8))
    bar=$(printf '█%.0s' $(seq 1 $((pct / 5))))
    empty=$(printf '░%.0s' $(seq 1 $((20 - pct / 5))))
    echo "[$bar$empty] $pct%"
    ;;
  [1-8])
    echo "═══════════════════════════════════════"
    echo "  复制以下内容到 Claude Code"
    echo "═══════════════════════════════════════"
    echo ""
    cat "$TASKS_DIR/task-$1.md"
    ;;
  *)
    echo "用法: ./run-task.sh <command>"
    echo ""
    echo "  list       查看任务列表"
    echo "  next       下一个待完成任务"
    echo "  <1-8>      输出任务 prompt"
    echo "  done <N>   标记完成"
    echo "  status     进度"
    ;;
esac
