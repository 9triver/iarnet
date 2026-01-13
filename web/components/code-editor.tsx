'use client'

import React, { useState, useEffect } from 'react'
import Editor, { loader } from '@monaco-editor/react'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { ChevronRight, ChevronDown, File, Folder, Code, Save, Plus, Trash } from 'lucide-react'
import { cn } from '@/lib/utils'
import { applicationsAPI } from '@/lib/api'
import { toast } from 'sonner'
import type { FileInfo } from '@/lib/model'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'


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
  onFileDelete: (path: string, isDir: boolean) => void
  onFileCreate: (path: string, isDir: boolean) => void
}

interface CreateDialogState {
  isOpen: boolean
  name: string
  type: "file" | "folder"
  targetPath: string
}

const FileTreeItem: React.FC<FileTreeItemProps> = ({
  file,
  level,
  onFileSelect,
  onFolderToggle,
  expandedFolders,
  selectedFile,
  onFileDelete,
  onFileCreate
}) => {
  const [createDialog, setCreateDialog] = useState<CreateDialogState>({
    isOpen: false,
    name: "",
    type: "file",
    targetPath: file.path
  })
  
  const isExpanded = expandedFolders.has(file.path)
  const isSelected = selectedFile === file.path
  
  const handleCreate = () => {
    if (createDialog.name.trim()) {
      onFileCreate(`${createDialog.targetPath}/${createDialog.name.trim()}`, createDialog.type === "folder")
      setCreateDialog({ isOpen: false, name: "", type: "file", targetPath: file.path })
    }
  }
  
  const handleClick = (e: React.MouseEvent) => {
    // 如果点击的是按钮或对话框相关元素，不触发文件/文件夹的点击事件
    const target = e.target as HTMLElement
    
    // 检查是否点击了交互元素（按钮、输入框等）
    const isInteractiveElement = target.closest('button') || 
      target.closest('input') ||
      target.closest('label') ||
      target.closest('[role="radiogroup"]') ||
      target.closest('[role="radio"]')
    
    // 检查是否点击了对话框（通过 Portal 渲染，可能在 DOM 树外部）
    const isDialogElement = target.closest('[role="dialog"]') || 
      target.closest('[data-radix-portal]')
    
    // 如果点击的是交互元素，阻止事件
    if (isInteractiveElement) {
      e.stopPropagation()
      e.preventDefault()
      return
    }
    
    // 如果点击的是对话框元素，阻止事件（但不在文件树项上检查，因为对话框通过 Portal 渲染）
    if (isDialogElement) {
      e.stopPropagation()
      e.preventDefault()
      return
    }
    
    // 正常处理文件/文件夹点击
    if (file.is_dir) {
      onFolderToggle(file.path)
    } else {
      onFileSelect(file.path)
    }
  }
  
  // 处理鼠标按下事件，提前阻止事件冒泡
  const handleMouseDown = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement
    
    // 只阻止按钮和输入框的事件冒泡
    if (
      target.closest('button') || 
      target.closest('input') ||
      target.closest('label') ||
      target.closest('[role="radiogroup"]') ||
      target.closest('[role="radio"]')
    ) {
      e.stopPropagation()
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
      onMouseDown={handleMouseDown}
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
      <div className="flex-grow" />
      {file.is_dir && (
        <div onClick={(e) => e.stopPropagation()} onMouseDown={(e) => e.stopPropagation()}>
          <AlertDialog open={createDialog.isOpen} onOpenChange={(open) => setCreateDialog({ ...createDialog, isOpen: open })}>
            <AlertDialogTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6 text-muted-foreground hover:bg-transparent hover:text-foreground"
                onClick={(e) => {
                  e.stopPropagation()
                  e.preventDefault()
                  setCreateDialog({ isOpen: true, name: "", type: "file", targetPath: file.path })
                }}
                onMouseDown={(e) => {
                  e.stopPropagation()
                  e.preventDefault()
                }}
              >
                <Plus className="h-3 w-3" />
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent onClick={(e) => e.stopPropagation()} onMouseDown={(e) => e.stopPropagation()}>
              <AlertDialogHeader>
                <AlertDialogTitle>创建新文件/文件夹</AlertDialogTitle>
                <AlertDialogDescription>
                  在 <span className="font-semibold">{file.path}</span> 中创建新文件或文件夹。
                </AlertDialogDescription>
              </AlertDialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor={`name-${file.path}`} className="text-right">
                  名称
                </Label>
                <Input
                  id={`name-${file.path}`}
                  placeholder="输入文件名或文件夹名"
                  value={createDialog.name}
                  onChange={(e) => setCreateDialog({ ...createDialog, name: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      handleCreate()
                    }
                  }}
                  className="col-span-3"
                  autoFocus
                />
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroup value={createDialog.type} onValueChange={(value) => setCreateDialog({ ...createDialog, type: value as "file" | "folder" })} className="flex items-center space-x-4">
                  <label htmlFor={`file-${file.path}`} className="flex items-center space-x-2 cursor-pointer">
                    <RadioGroupItem value="file" id={`file-${file.path}`} />
                    <span className="font-normal">文件</span>
                  </label>
                  <label htmlFor={`folder-${file.path}`} className="flex items-center space-x-2 cursor-pointer">
                    <RadioGroupItem value="folder" id={`folder-${file.path}`} />
                    <span className="font-normal">文件夹</span>
                  </label>
                </RadioGroup>
              </div>
            </div>
            <AlertDialogFooter>
              <AlertDialogCancel onClick={() => setCreateDialog({ isOpen: false, name: "", type: "file", targetPath: file.path })}>取消</AlertDialogCancel>
              <AlertDialogAction onClick={handleCreate}>创建</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        </div>
      )}
      <div onClick={(e) => e.stopPropagation()} onMouseDown={(e) => e.stopPropagation()}>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 text-muted-foreground hover:bg-transparent hover:text-foreground"
              onClick={(e) => {
                e.stopPropagation()
                e.preventDefault()
              }}
              onMouseDown={(e) => {
                e.stopPropagation()
                e.preventDefault()
              }}
            >
              <Trash className="h-3 w-3" />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent onClick={(e) => e.stopPropagation()} onMouseDown={(e) => e.stopPropagation()}>
            <AlertDialogHeader>
              <AlertDialogTitle>确认删除</AlertDialogTitle>
              <AlertDialogDescription>
                您确定要删除 <span className="font-semibold">{file.path}</span> 吗？此操作不可撤销。
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>取消</AlertDialogCancel>
              <AlertDialogAction onClick={() => onFileDelete(file.path, file.is_dir)}>删除</AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
          </div>
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
  const [editorContent, setEditorContent] = useState<string | undefined>(undefined)
  const [isDirty, setIsDirty] = useState(false)
  const [isRootCreateDialogOpen, setIsRootCreateDialogOpen] = useState(false)
  const [rootCreateName, setRootCreateName] = useState("")
  const [rootCreateType, setRootCreateType] = useState<"file" | "folder">("file")

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
       setEditorContent(result.content)
       setSelectedFilePath(filePath)
       setIsDirty(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  // 保存文件内容
  const handleSaveFile = async () => {
    if (!currentFile || editorContent === undefined) return

    try {
      setLoading(true)
      await applicationsAPI.saveFileContent(appId, currentFile.path, editorContent)
      toast.success('文件保存成功')
      setIsDirty(false)
      // 重新获取文件内容以更新mod_time等信息
      fetchFileContent(currentFile.path)
    } catch (err) {
      toast.error('文件保存失败', { description: err instanceof Error ? err.message : '未知错误' })
    } finally {
      setLoading(false)
    }
  }

  // 创建文件/目录
  const handleCreateFileOrDirectory = async (path: string, isDir: boolean) => {
    try {
      setLoading(true)
      if (isDir) {
        await applicationsAPI.createDirectory(appId, path)
        toast.success(`目录 ${path} 创建成功`)
      } else {
        await applicationsAPI.createFile(appId, path)
        toast.success(`文件 ${path} 创建成功`)
      }
      // 刷新父目录的文件树
      const parentPath = path.substring(0, path.lastIndexOf('/')) || '/'
      fetchFileTree(parentPath)
    } catch (err) {
      toast.error('创建失败', { description: err instanceof Error ? err.message : '未知错误' })
    } finally {
      setLoading(false)
    }
  }

  // 删除文件/目录
  const handleDeleteFileOrDirectory = async (path: string, isDir: boolean) => {
    try {
      setLoading(true)
      if (isDir) {
        await applicationsAPI.deleteDirectory(appId, path)
        toast.success(`目录 ${path} 删除成功`)
      } else {
        await applicationsAPI.deleteFile(appId, path)
        toast.success(`文件 ${path} 删除成功`)
      }
      // 刷新父目录的文件树
      const parentPath = path.substring(0, path.lastIndexOf('/')) || '/'
      fetchFileTree(parentPath)
      // 如果删除的是当前打开的文件，则清空编辑器
      if (selectedFilePath === path) {
        setCurrentFile(null)
        setSelectedFilePath(undefined)
        setEditorContent(undefined)
        setIsDirty(false)
      }
    } catch (err) {
      toast.error('删除失败', { description: err instanceof Error ? err.message : '未知错误' })
    } finally {
      setLoading(false)
    }
  }

  // 处理文件选择
  const handleFileSelect = (filePath: string) => {
    if (isDirty) {
      // 提示用户保存或放弃更改
      // 暂时直接切换，后续可以添加确认弹窗
      console.warn('当前文件有未保存的更改，已放弃。')
    }
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
    
    // 如果是根目录，添加根目录创建按钮
    if (path === '/') {
      result.push(
        <div 
          key="root-create" 
          className="flex items-center gap-1 px-2 py-1 text-sm"
          onClick={(e) => {
            // 如果根目录对话框打开，阻止事件冒泡
            if (isRootCreateDialogOpen) {
              e.stopPropagation()
              e.preventDefault()
            }
          }}
          onMouseDown={(e) => {
            if (isRootCreateDialogOpen) {
              e.stopPropagation()
            }
          }}
        >
          <Folder className="h-4 w-4 text-blue-500" />
          <span className="flex-1 text-muted-foreground">/</span>
          <div onClick={(e) => e.stopPropagation()} onMouseDown={(e) => e.stopPropagation()}>
            <AlertDialog open={isRootCreateDialogOpen} onOpenChange={setIsRootCreateDialogOpen}>
              <AlertDialogTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 text-muted-foreground hover:bg-transparent hover:text-foreground"
                  onClick={(e) => {
                    e.stopPropagation()
                    e.preventDefault()
                    setIsRootCreateDialogOpen(true)
                  }}
                  onMouseDown={(e) => {
                    e.stopPropagation()
                    e.preventDefault()
                  }}
                >
                  <Plus className="h-3 w-3" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent onClick={(e) => e.stopPropagation()} onMouseDown={(e) => e.stopPropagation()}>
                <AlertDialogHeader>
                <AlertDialogTitle>创建新文件/文件夹</AlertDialogTitle>
                <AlertDialogDescription>
                  在根目录中创建新文件或文件夹。
                </AlertDialogDescription>
              </AlertDialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label htmlFor="root-create-name" className="text-right">
                    名称
                  </Label>
                  <Input
                    id="root-create-name"
                    placeholder="输入文件名或文件夹名"
                    value={rootCreateName}
                    onChange={(e) => setRootCreateName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        handleCreateAtRoot()
                      }
                    }}
                    className="col-span-3"
                    autoFocus
                  />
                </div>
                <div className="flex items-center space-x-2">
                  <RadioGroup value={rootCreateType} onValueChange={(value) => setRootCreateType(value as "file" | "folder")} className="flex items-center space-x-4">
                    <label htmlFor="root-file" className="flex items-center space-x-2 cursor-pointer">
                      <RadioGroupItem value="file" id="root-file" />
                      <span className="font-normal">文件</span>
                    </label>
                    <label htmlFor="root-folder" className="flex items-center space-x-2 cursor-pointer">
                      <RadioGroupItem value="folder" id="root-folder" />
                      <span className="font-normal">文件夹</span>
                    </label>
                  </RadioGroup>
                </div>
              </div>
              <AlertDialogFooter>
                <AlertDialogCancel onClick={() => {
                  setRootCreateName("")
                  setRootCreateType("file")
                }}>取消</AlertDialogCancel>
                <AlertDialogAction onClick={handleCreateAtRoot}>
                  创建
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
          </div>
        </div>
      )
    }
    
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
          onFileDelete={handleDeleteFileOrDirectory}
          onFileCreate={handleCreateFileOrDirectory}
        />
      )
      
      // 如果是文件夹且已展开，且该文件夹的数据已加载，递归渲染子文件
      if (file.is_dir && expandedFolders.has(file.path) && fileTree.has(file.path)) {
        result.push(...renderFileTree(file.path, level + 1))
      }
    })
    
    return result
  }

  // 配置 Monaco Editor 使用本地资源（解决 Docker 容器中 Worker 加载问题）
  useEffect(() => {
    // 配置 Monaco Editor 使用本地路径而不是 CDN
    // 在 Next.js 中，Monaco Editor 的资源会被打包到 _next/static 目录
    if (typeof window !== 'undefined') {
      loader.config({ 
        paths: { 
          vs: '/_next/static/chunks/node_modules_monaco-editor_min_vs' 
        } 
      })
      
      // 如果上面的路径不工作，尝试使用 CDN 但设置超时
      // 或者使用 monaco-editor 包中的路径
      try {
        // 尝试从 node_modules 加载（开发环境）
        const monacoPath = '/node_modules/monaco-editor/min/vs'
        // 或者使用 CDN 作为后备
        loader.config({ 
          paths: { 
            vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.52.2/min/vs'
          } 
        })
      } catch (error) {
        console.warn('Monaco Editor loader config failed:', error)
      }
    }
  }, [])

  // 初始化加载根目录
  useEffect(() => {
    fetchFileTree()
  }, [appId])

  // 在根目录创建文件/目录
  const handleCreateAtRoot = async () => {
    if (!rootCreateName.trim()) {
      toast.error("请输入名称")
      return
    }
    try {
      await handleCreateFileOrDirectory(rootCreateName.trim(), rootCreateType === "folder")
      setIsRootCreateDialogOpen(false)
      setRootCreateName("")
      setRootCreateType("file")
    } catch (error) {
      // 错误已在 handleCreateFileOrDirectory 中处理
    }
  }

  return (
    <div className={cn('flex h-full border rounded-lg overflow-hidden', className)}>
      {/* 文件树侧边栏 */}
      <div className="w-80 border-r bg-muted/30">
        <ScrollArea className="h-full">
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
            <div className="p-3 border-b bg-background flex items-center justify-between">
              <div className="flex items-center gap-2">
                <File className="h-4 w-4" />
                <span className="font-medium">{currentFile.path}</span>
                {isDirty && <span className="text-sm text-orange-500">(未保存)</span>}
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleSaveFile}
                disabled={!isDirty || loading}
              >
                <Save className="h-4 w-4 mr-2" />
                保存
              </Button>
            </div>
            
            {/* Monaco Editor */}
            <div className="flex-1">
              <Editor
                height="100%"
                language={currentFile.language}
                value={editorContent}
                theme="vs"
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
                onMount={(editor) => {
                  editor.onDidChangeModelContent(() => {
                    setEditorContent(editor.getValue())
                    setIsDirty(true)
                  })
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