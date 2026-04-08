#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
代码行数统计工具
支持统计Go、React/JavaScript/TypeScript等项目的代码行数
"""

import os
import sys
import locale
from pathlib import Path
from collections import defaultdict

# 设置控制台编码
if sys.platform == 'win32':
    sys.stdout.reconfigure(encoding='utf-8')
    sys.stderr.reconfigure(encoding='utf-8')

# 文件扩展名映射到编程语言
LANGUAGE_EXTENSIONS = {
    # Go files
    '.go': 'Go',
    # React/JavaScript/TypeScript files
    '.js': 'JavaScript',
    '.jsx': 'React',
    '.ts': 'TypeScript',
    '.tsx': 'React/TypeScript',
    # Common web files
    '.html': 'HTML',
    '.css': 'CSS',
    '.scss': 'SCSS',
    # JSON files
    '.json': 'JSON',
    '.json5': 'JSON',
}

def count_lines_in_file(file_path):
    """统计文件行数"""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()
        return len(lines)
    except (UnicodeDecodeError, PermissionError, FileNotFoundError):
        return 0

def scan_directory(directory):
    """递归扫描目录并统计代码行数"""
    results = defaultdict(lambda: [0, 0])  # [行数, 文件数]
    
    for root, dirs, files in os.walk(directory):
        # 排除node_modules文件夹
        if 'node_modules' in dirs:
            dirs.remove('node_modules')
        
        for file in files:
            file_path = Path(root) / file
            extension = file_path.suffix.lower()
            
            if extension in LANGUAGE_EXTENSIONS:
                language = LANGUAGE_EXTENSIONS[extension]
                line_count = count_lines_in_file(file_path)
                
                results[language][0] += line_count  # 增加行数
                results[language][1] += 1         # 增加文件数
    
    return results

def main():
    # 获取命令行参数
    if len(sys.argv) > 1:
        target_dir = sys.argv[1]
    else:
        print("请提供目标文件夹路径")
        return
    
    path = Path(target_dir)
    
    # 检查路径是否存在
    if not path.exists():
        print(f"文件夹不存在: {target_dir}")
        return
    
    if not path.is_dir():
        print(f"提供的路径不是一个文件夹: {target_dir}")
        return
    
    print("扫描文件夹: " + target_dir)
    print("=" * 50)
    print()
    
    # 扫描目录
    results = scan_directory(target_dir)
    
    # 打印结果
    for language, (lines, files) in sorted(results.items()):
        print(language + " - 文件数: " + str(files) + ", 总行数: " + str(lines))
    
    print("\n" + "=" * 50)
    
    # 计算总体统计
    total_lines = sum(lines for lines, _ in results.values())
    total_files = sum(files for _, files in results.values())
    
    print("总体统计:")
    print("   总文件数: " + str(total_files))
    print("   总代码行数: " + str(total_lines))
    
    # 重点关注React和Go项目
    print("\n重点关注:")
    
    react_lines = results.get('React', [0, 0])[0] + results.get('React/TypeScript', [0, 0])[0]
    react_files = results.get('React', [0, 0])[1] + results.get('React/TypeScript', [0, 0])[1]
    go_lines = results.get('Go', [0, 0])[0]
    go_files = results.get('Go', [0, 0])[1]
    
    print("   React 项目 - 文件数: " + str(react_files) + ", 总行数: " + str(react_lines))
    print("   Go 项目 - 文件数: " + str(go_files) + ", 总行数: " + str(go_lines))

if __name__ == "__main__":
    main()