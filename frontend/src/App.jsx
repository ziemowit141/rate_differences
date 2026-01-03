import { useEffect, useState } from 'react'
import './App.css'

function App() {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [uploadResults, setUploadResults] = useState([])
  const [deleting, setDeleting] = useState('')

  const loadTransactions = async () => {
    setLoading(true)
    setError('')
    try {
      const response = await fetch('/transactions')
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const payload = await response.json()
      setData(payload)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  const handleUpload = async (event) => {
    const files = Array.from(event.target.files || [])
    if (files.length === 0) {
      return
    }
    await uploadFiles(files)
    event.target.value = ''
  }

  const uploadFiles = async (files) => {
    if (!files.length) return
    setUploading(true)
    setUploadError('')
    setUploadResults([])
    try {
      const formData = new FormData()
      files.forEach((file) => formData.append('files', file))
      const response = await fetch('/upload', {
        method: 'POST',
        body: formData,
      })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const payload = await response.json()
      setUploadResults(payload.files || [])
      await loadTransactions()
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setUploading(false)
    }
  }

  const handleDrop = async (event) => {
    event.preventDefault()
    const files = Array.from(event.dataTransfer.files || []).filter(
      (file) => file.type === 'application/pdf'
    )
    if (!files.length || uploading) return
    await uploadFiles(files)
  }

  const handleDragOver = (event) => {
    event.preventDefault()
  }

  const handleDelete = async (baseName) => {
    if (!baseName || deleting) return
    setDeleting(baseName)
    setError('')
    try {
      const response = await fetch(`/files/${encodeURIComponent(baseName)}`, {
        method: 'DELETE',
      })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      await loadTransactions()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setDeleting('')
    }
  }

  useEffect(() => {
    void loadTransactions()
  }, [])

  return (
    <div className="app">
      <header className="app-header">
        <div>
          <p className="eyebrow">MT940 Viewer</p>
          <h1>Transactions from your statements</h1>
          <p className="subhead">
            This view pulls data from the Go API at <code>/transactions</code>.
          </p>
        </div>
        <div className="header-actions">
          <label className="upload">
            <input
              type="file"
              accept="application/pdf"
              multiple
              onChange={handleUpload}
              disabled={uploading}
            />
            <span className="button button-secondary">
              {uploading ? 'Uploading…' : 'Upload PDFs'}
            </span>
          </label>
          <button className="button" onClick={loadTransactions} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </header>

      <section
        className={`dropzone ${uploading ? 'dropzone-disabled' : ''}`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
      >
        <div>
          <h2>Drag & drop PDFs</h2>
          <p>Drop multiple Citibank statements here or use the upload button.</p>
        </div>
      </section>

      {uploadError && (
        <div className="panel panel-error">
          <strong>Upload failed:</strong> {uploadError}
        </div>
      )}

      {uploadResults.length > 0 && (
        <div className="panel panel-upload">
          <h3>Upload results</h3>
          <ul>
            {uploadResults.map((result) => (
              <li key={result.source}>
                <strong>{result.source}</strong>{' '}
                {result.error ? (
                  <span className="error-text">{result.error}</span>
                ) : (
                  <span>
                    → {result.pdf_path} → {result.mt940_path}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {error && (
        <div className="panel panel-error">
          <strong>Load failed:</strong> {error}
        </div>
      )}

      {!data && !loading && !error && (
        <div className="panel">No data yet. Click refresh to try again.</div>
      )}

      {data?.files?.length ? (
        <div className="files">
          {data.files.map((file) => (
            <section key={file.file} className="file-card">
              <div className="file-header">
                <div>
                  <p className="file-label">Statement file</p>
                  <h2>{file.base_name || file.file}</h2>
                </div>
                <div className="file-actions">
                  <span className="pill">
                    {file.transactions?.length ?? 0} transactions
                  </span>
                  <button
                    className="button button-tertiary"
                    onClick={() => handleDelete(file.base_name)}
                    disabled={deleting === file.base_name}
                  >
                    {deleting === file.base_name ? 'Deleting…' : 'Delete'}
                  </button>
                </div>
              </div>
              {file.error ? (
                <div className="panel panel-error">{file.error}</div>
              ) : (
                <div className="table">
                  <div className="table-row table-head">
                    <span>Value date</span>
                    <span>Entry date</span>
                    <span>D/C</span>
                    <span>Amount</span>
                    <span>Code</span>
                    <span>Reference</span>
                    <span>Details</span>
                  </div>
                  {file.transactions?.map((txn, index) => (
                    <div className="table-row" key={`${file.file}-${index}`}>
                      <span>{txn.value_date || '—'}</span>
                      <span>{txn.entry_date || '—'}</span>
                      <span>{txn.dc_mark || '—'}</span>
                      <span>{txn.amount || '—'}</span>
                      <span>{txn.code || '—'}</span>
                      <span className="mono">{txn.reference || '—'}</span>
                      <span className="details">{txn.details || '—'}</span>
                    </div>
                  ))}
                </div>
              )}
            </section>
          ))}
        </div>
      ) : null}
    </div>
  )
}

export default App
