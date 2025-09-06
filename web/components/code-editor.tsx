'use client'

import React, { useState, useEffect } from 'react'
import Editor from '@monaco-editor/react'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { ChevronRight, ChevronDown, File, Folder, Code } from 'lucide-react'
import { cn } from '@/lib/utils'
import { applicationsAPI } from '@/lib/api'


interface FileContent {
  path: string
  content: string
  size: number
  mod_time: string
  language: string
}

interface CodeEditorProps {
  appId: string
  className?: string
}

interface FileTreeItemProps {
  file: FileInfo
  level: number
  onFileSelect: (path: string) => void
  onFolderToggle: (path: string) => void
  expandedFolders: Set<string>
  selectedFile?: string
}

const FileTreeItem: React.FC<FileTreeItemProps> = ({
  file,
  level,
  onFileSelect,
  onFolderToggle,
  expandedFolders,
  selectedFile
}) => {
  const isExpanded = expandedFolders.has(file.path)
  const isSelected = selectedFile === file.path
  
  const handleClick = () => {
    if (file.is_dir) {
      onFolderToggle(file.path)
    } else {
      onFileSelect(file.path)
    }
  }

  return (
    <div
      className={cn(
        'flex items-center gap-1 px-2 py-1 text-sm cursor-pointer hover:bg-accent rounded-sm',
        isSelected && 'bg-accent',
        'transition-colors'
      )}
      style={{ paddingLeft: `${level * 12 + 8}px` }}
      onClick={handleClick}
    >
      {file.is_dir ? (
        <>
          {isExpanded ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
          <Folder className="h-4 w-4 text-blue-500" />
        </>
      ) : (
        <>
          <div className="w-4" /> {/* Spacer for alignment */}
          <File className="h-4 w-4 text-muted-foreground" />
        </>
      )}
      <span className="truncate">{file.name}</span>
    </div>
  )
}

export const CodeEditor: React.FC<CodeEditorProps> = ({ appId, className }) => {
  const [fileTree, setFileTree] = useState<Map<string, FileInfo[]>>(new Map())
  const [currentFile, setCurrentFile] = useState<FileContent | null>(null)
  const [selectedFilePath, setSelectedFilePath] = useState<string | undefined>()
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set(['/']))
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // 获取文件树
  const fetchFileTree = async (path: string = '/') => {
    try {
      setLoading(true)
      const result = await applicationsAPI.getFileTree(appId, path)
      console.log('获取文件树:', result)
      setFileTree(prev => {
        const newTree = new Map(prev)
        newTree.set(path, result.files)
        console.log('更新后的文件树:', newTree) // 在这里打印更新后的状态
        return newTree
      })
      // 移除这行，因为这里的 fileTree 还是旧的状态值
      // console.log('文件树:', fileTree)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  // 获取文件内容
  const fetchFileContent = async (filePath: string) => {
    try {
      setLoading(true)
      const result = await applicationsAPI.getFileContent(appId, filePath)
       setCurrentFile({ 
         content: result.content, 
         path: result.path,
         size: 0, // 文件内容接口不返回size
         mod_time: '', // 文件内容接口不返回mod_time
         language: result.language
       })
       setSelectedFilePath(filePath)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  // 处理文件选择
  const handleFileSelect = (filePath: string) => {
    fetchFileContent(filePath)
  }

  // 处理文件夹展开/收起
  const handleFolderToggle = (folderPath: string) => {
    const newExpanded = new Set(expandedFolders)
    if (newExpanded.has(folderPath)) {
      newExpanded.delete(folderPath)
    } else {
      newExpanded.add(folderPath)
      // 如果是首次展开，获取该文件夹的内容
      if (!fileTree.has(folderPath)) {
        fetchFileTree(folderPath)
      }
    }
    setExpandedFolders(newExpanded)
  }

  // 递归渲染文件树
  const renderFileTree = (path: string = '/', level: number = 0): React.ReactNode[] => {
    const files = fileTree.get(path) || []
    const result: React.ReactNode[] = []
    
    files.forEach((file) => {
      result.push(
        <FileTreeItem
          key={file.path}
          file={file}
          level={level}
          onFileSelect={handleFileSelect}
          onFolderToggle={handleFolderToggle}
          expandedFolders={expandedFolders}
          selectedFile={selectedFilePath}
        />
      )
      
      // 如果是文件夹且已展开，且该文件夹的数据已加载，递归渲染子文件
      if (file.is_dir && expandedFolders.has(file.path) && fileTree.has(file.path)) {
        result.push(...renderFileTree(file.path, level + 1))
      }
    })
    
    return result
  }

  // 初始化加载根目录
  useEffect(() => {
    fetchFileTree()
  }, [appId])

  return (
    <div className={cn('flex h-full border rounded-lg overflow-hidden', className)}>
      {/* 文件树侧边栏 */}
      <div className="w-80 border-r bg-muted/30">
        {/* <div className="p-3 border-b">
           <div className="flex items-center gap-2">
            <Code className="h-5 w-5" />
            <h3 className="font-semibold">文件浏览器</h3>
          </div> 
        </div> */}
        <ScrollArea className="h-[calc(100%-60px)]">
          <div className="p-2">
            {loading && fileTree.size === 0 ? (
              <div className="text-sm text-muted-foreground p-2">加载中...</div>
            ) : error ? (
              <div className="text-sm text-destructive p-2">{error}</div>
            ) : (
              renderFileTree()
            )}
          </div>
        </ScrollArea>
      </div>

      {/* 代码编辑器主区域 */}
      <div className="flex-1 flex flex-col">
        {currentFile ? (
          <>
            {/* 文件标签栏 */}
            <div className="p-3 border-b bg-background">
              <div className="flex items-center gap-2">
                <File className="h-4 w-4" />
                <span className="font-medium">{currentFile.path}</span>
                <span className="text-sm text-muted-foreground">({currentFile.size} bytes)</span>
              </div>
            </div>
            
            {/* Monaco Editor */}
            <div className="flex-1">
              <Editor
                height="100%"
                language={currentFile.language}
                value={currentFile.content}
                theme="vs-dark"
                options={{
                  readOnly: false,
                  minimap: { enabled: true },
                  fontSize: 14,
                  lineNumbers: 'on',
                  roundedSelection: false,
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                  wordWrap: 'on',
                  folding: true,
                  lineDecorationsWidth: 10,
                  lineNumbersMinChars: 3,
                  glyphMargin: false,
                }}
                loading={<div className="flex items-center justify-center h-full">加载编辑器...</div>}
              />
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-muted-foreground">
            <div className="text-center">
              <Code className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p className="text-lg font-medium mb-2">选择一个文件开始编辑</p>
              <p className="text-sm">从左侧文件树中选择要查看或编辑的文件</p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default CodeEditor