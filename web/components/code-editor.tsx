'use client'

import React, { useState, useEffect } from 'react'
import Editor from '@monaco-editor/react'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { ChevronRight, ChevronDown, File, Folder, Code, Save, Plus, Trash } from 'lucide-react'
import { cn } from '@/lib/utils'
import { applicationsAPI } from '@/lib/api'
import { toast } from 'sonner'
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
      <div className="flex-grow" />
      {file.is_dir && (
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 text-muted-foreground hover:bg-transparent hover:text-foreground"
              onClick={(e) => e.stopPropagation()}
            >
              <Plus className="h-3 w-3" />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>创建新文件/文件夹</AlertDialogTitle>
              <AlertDialogDescription>
                在 <span className="font-semibold">{file.path}</span> 中创建新文件或文件夹。
              </AlertDialogDescription>
            </AlertDialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="name" className="text-right">
                  名称
                </Label>
                <Input id="name" defaultValue="" className="col-span-3" />
              </div>
              <div className="flex items-center space-x-2">
                <input type="radio" id="file" name="type" value="file" defaultChecked />
                <Label htmlFor="file">文件</Label>
                <input type="radio" id="folder" name="type" value="folder" />
                <Label htmlFor="folder">文件夹</Label>
              </div>
            </div>
            <AlertDialogFooter>
              <AlertDialogCancel>取消</AlertDialogCancel>
              <AlertDialogAction onClick={() => {
                const nameInput = document.getElementById('name') as HTMLInputElement
                const typeInputs = document.getElementsByName('type') as NodeListOf<HTMLInputElement>
                let type = 'file'
                typeInputs.forEach(input => {
                  if (input.checked) {
                    type = input.value
                  }
                })
                if (nameInput.value) {
                  onFileCreate(`${file.path}/${nameInput.value}`, type === 'folder')
                }
              }}>创建</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      )}
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 text-muted-foreground hover:bg-transparent hover:text-foreground"
            onClick={(e) => e.stopPropagation()}
          >
            <Trash className="h-3 w-3" />
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
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