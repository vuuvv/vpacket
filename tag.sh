#!/bin/bash

# 显示所有执行命令
set -ex

go mod tidy

# 获取远程仓库中的最新标签
latest_tag=$(git ls-remote --tags origin | awk '{print $2}' | cut -d '/' -f 3 | grep -Eo '[0-9]+\.[0-9]+\.[0-9]+' | /bin/sort -V | tail -n 1)

if [ -z "$latest_tag" ]; then
  # 如果当前没有标签，则创建一个新的标签
  new_tag="v0.0.1"
else
  # 如果当前有标签，则解析出版本号并递增次版本号
  major=$(echo "$latest_tag" | cut -d. -f1)
  minor=$(echo "$latest_tag" | cut -d. -f2)
  patch=$(echo "$latest_tag" | cut -d. -f3)
  patch=$((patch + 1))
  new_tag="v$major.$minor.$patch"
fi

# 将 pubspec.yaml 和 version 文件提交到 git
git add -A .
git commit -m "chore: bump version to $new_tag" || true

# 在本地创建新的标签
git tag "$new_tag"

# 推送新的标签到远程仓库
git push origin tag "$new_tag"

# 推送新的提交到远程仓库
default_branch=$(git symbolic-ref --short HEAD)
git push origin "$default_branch"

# 输出新的标签名称
echo "已创建新标签：$new_tag"
