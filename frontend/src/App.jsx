import { useEffect, useState } from 'react'
import './App.css'

function App() {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

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
          <button className="button" onClick={loadTransactions} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </header>

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
                  <h2>{file.file}</h2>
                </div>
                <span className="pill">
                  {file.transactions?.length ?? 0} transactions
                </span>
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
