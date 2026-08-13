import { File, Folder, Link2Off } from 'lucide-react'
import { useApp } from '../app/use-app'

export function SitesPage() {
  const { files, browse, project } = useApp()

  return (
    <div className="page-plain">
      <header className="page-heading">
        <h1>站点</h1>
        <p>{project?.rootPath ?? '尚未打开项目。文件浏览限制在项目根目录内。'}</p>
      </header>
      <div className="file-list">
        {files.length === 0 && <p className="page-muted">没有可显示的文件。</p>}
        {files.map((file) => (
          <button
            key={file.path}
            type="button"
            className={`file-row ${file.externalSymlink ? 'blocked' : ''}`}
            onClick={() => file.dir && void browse(file.path)}
            title={file.externalSymlink ? '外部符号链接已被工作区边界拦截' : file.path}
          >
            {file.externalSymlink ? <Link2Off size={15} strokeWidth={1.75} /> : file.dir ? <Folder size={15} strokeWidth={1.75} /> : <File size={15} strokeWidth={1.75} />}
            {file.name}
            {file.externalSymlink && <span className="badge">已拦截</span>}
          </button>
        ))}
      </div>
    </div>
  )
}
