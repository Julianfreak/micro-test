import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  ArrowUpRight,
  Boxes,
  Check,
  ChevronDown,
  CircleHelp,
  Clock3,
  LayoutDashboard,
  LoaderCircle,
  LogOut,
  Package,
  Pencil,
  Plus,
  Search,
  ShieldCheck,
  Sparkles,
  Trash2,
  TrendingUp,
  X,
} from 'lucide-react'
import { ApiError, api, type InventoryItem } from './api'

const EMPTY_ITEMS: InventoryItem[] = []

function App() {
  const queryClient = useQueryClient()
  const session = useQuery({
    queryKey: ['session'],
    queryFn: api.session,
    retry: false,
  })
  const login = useMutation({
    mutationFn: api.login,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['session'] })
    },
  })

  if (session.isPending) {
    return <LoadingScreen />
  }

  if (session.isError && !(session.error instanceof ApiError && session.error.status === 401)) {
    return <ServiceUnavailable onRetry={() => void session.refetch()} />
  }

  if (!session.isSuccess) {
    return (
      <LoginScreen
        isLoading={login.isPending}
        error={login.error instanceof Error ? login.error.message : undefined}
        onSubmit={(username, password) => login.mutate({ username, password })}
      />
    )
  }

  return <InventoryDashboard />
}

function LoadingScreen() {
  return (
    <main className="loading-screen">
      <Brand />
      <LoaderCircle className="spin" size={26} aria-label="Cargando" />
    </main>
  )
}

function ServiceUnavailable({ onRetry }: { onRetry: () => void }) {
  return (
    <main className="login-shell">
      <section className="login-card">
        <Brand />
        <div className="service-icon">
          <AlertCircle size={23} />
        </div>
        <h1>No pudimos conectar</h1>
        <p>Revisa que la API esté disponible e inténtalo de nuevo.</p>
        <button className="button button-primary button-wide" onClick={onRetry}>
          Volver a intentar
        </button>
      </section>
    </main>
  )
}

function LoginScreen({
  isLoading,
  error,
  onSubmit,
}: {
  isLoading: boolean
  error?: string
  onSubmit: (username: string, password: string) => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    onSubmit(username.trim(), password)
  }

  return (
    <main className="login-shell">
      <div className="login-decoration decoration-one" />
      <div className="login-decoration decoration-two" />
      <section className="login-card">
        <Brand />
        <div className="login-heading">
          <div className="eyebrow"><ShieldCheck size={14} /> ACCESO SEGURO</div>
          <h1>Qué bueno tenerte<br />de vuelta.</h1>
          <p>Inicia sesión para administrar tu inventario.</p>
        </div>
        <form className="login-form" onSubmit={submit}>
          <label htmlFor="username">Usuario</label>
          <input
            autoComplete="username"
            autoFocus
            id="username"
            name="username"
            onChange={(event) => setUsername(event.target.value)}
            placeholder="Tu usuario"
            required
            value={username}
          />
          <label htmlFor="password">Contraseña</label>
          <input
            autoComplete="current-password"
            id="password"
            name="password"
            onChange={(event) => setPassword(event.target.value)}
            placeholder="Tu contraseña"
            required
            type="password"
            value={password}
          />
          {error && <p className="form-error" role="alert">{error}</p>}
          <button className="button button-primary button-wide login-submit" disabled={isLoading}>
            {isLoading ? <LoaderCircle className="spin" size={17} /> : null}
            {isLoading ? 'Verificando…' : 'Iniciar sesión'}
            {!isLoading && <ArrowUpRight size={17} />}
          </button>
        </form>
        <div className="login-footnote"><ShieldCheck size={14} /> Sesión protegida con cifrado</div>
      </section>
      <p className="login-caption">INVENTARIO, BAJO CONTROL.</p>
    </main>
  )
}

function InventoryDashboard() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [dialogItem, setDialogItem] = useState<InventoryItem | null | undefined>()
  const [deleteItem, setDeleteItem] = useState<InventoryItem | null>(null)
  const [notice, setNotice] = useState('')
  const items = useQuery({
    queryKey: ['items'],
    queryFn: api.listItems,
  })
  const saveItem = useMutation({
    mutationFn: (item: InventoryItem) =>
      dialogItem ? api.updateItem(dialogItem.sku, item) : api.createItem(item),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['items'] })
      setDialogItem(undefined)
      setNotice(dialogItem ? 'Artículo actualizado correctamente.' : 'Artículo agregado al inventario.')
    },
  })
  const removeItem = useMutation({
    mutationFn: (sku: string) => api.deleteItem(sku),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['items'] })
      setDeleteItem(null)
      setNotice('Artículo eliminado del inventario.')
    },
  })
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: async () => {
      queryClient.removeQueries({ queryKey: ['items'] })
      await queryClient.invalidateQueries({ queryKey: ['session'] })
    },
  })

  const inventory = items.data ?? EMPTY_ITEMS
  const filteredItems = useMemo(() => {
    const term = search.trim().toLocaleLowerCase()
    if (!term) return inventory
    return inventory.filter((item) =>
      `${item.name} ${item.sku}`.toLocaleLowerCase().includes(term),
    )
  }, [inventory, search])

  const totalUnits = inventory.reduce((total, item) => total + item.quantity, 0)
  const lowStock = inventory.filter((item) => item.quantity > 0 && item.quantity <= 5).length
  const outOfStock = inventory.filter((item) => item.quantity === 0).length
  const error = items.error ?? saveItem.error ?? removeItem.error ?? logout.error

  useEffect(() => {
    if (error instanceof ApiError && error.status === 401) {
      void queryClient.invalidateQueries({ queryKey: ['session'] })
    }
  }, [error, queryClient])

  function submitItem(item: InventoryItem) {
    saveItem.mutate(item)
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Brand />
        <div className="workspace-switcher">
          <div className="workspace-avatar">M</div>
          <div className="workspace-copy"><strong>Mi inventario</strong><span>Espacio de trabajo</span></div>
          <ChevronDown size={16} />
        </div>
        <div className="sidebar-label">MENÚ PRINCIPAL</div>
        <nav className="side-nav" aria-label="Navegación principal">
          <a className="side-link" href="#resumen"><LayoutDashboard size={18} /> Resumen</a>
          <a className="side-link active" href="#inventario"><Package size={18} /> Inventario <span className="nav-count">{inventory.length}</span></a>
        </nav>
        <div className="sidebar-bottom">
          <div className="help-card">
            <div className="help-icon"><CircleHelp size={17} /></div>
            <strong>¿Necesitas ayuda?</strong>
            <p>Estamos aquí para ayudarte a organizarte.</p>
            <a href="/swagger/index.html" rel="noreferrer" target="_blank">Ver documentación <ArrowUpRight size={14} /></a>
          </div>
          <div className="profile">
            <div className="profile-avatar">OP</div>
            <div className="profile-copy"><strong>Cuenta activa</strong><span>Sesión segura</span></div>
            <button
              aria-label="Cerrar sesión"
              className="icon-button profile-logout"
              disabled={logout.isPending}
              onClick={() => logout.mutate()}
              title="Cerrar sesión"
            >
              {logout.isPending ? <LoaderCircle className="spin" size={17} /> : <LogOut size={17} />}
            </button>
          </div>
        </div>
      </aside>

      <main className="main-content" id="resumen">
        <header className="topbar">
          <div className="breadcrumbs"><span>Workspace</span><span className="breadcrumb-divider">/</span><strong>Inventario</strong></div>
          <div className="topbar-right">
            <div className="live-status"><span /> Sistema operativo</div>
            <div className="topbar-date"><Clock3 size={15} /> Vista general</div>
          </div>
        </header>

        <section className="page-content" id="inventario">
          <div className="welcome-row">
            <div>
              <div className="eyebrow"><Sparkles size={14} /> TU CENTRO DE OPERACIONES</div>
              <h1>Todo en su lugar.</h1>
              <p className="page-subtitle">Una vista clara de lo que tienes y lo que necesitas.</p>
            </div>
            <button className="button button-primary" onClick={() => { setNotice(''); setDialogItem(null) }}>
              <Plus size={17} /> Nuevo artículo
            </button>
          </div>

          {notice && (
            <div className="notice" role="status">
              <Check size={17} /> {notice}
              <button aria-label="Cerrar aviso" className="icon-button" onClick={() => setNotice('')}><X size={16} /></button>
            </div>
          )}
          {error && (
            <div className="error-banner" role="alert">
              <AlertCircle size={17} />
              <span>{error instanceof Error ? error.message : 'Ocurrió un error inesperado.'}</span>
              {items.isError && <button className="text-button" onClick={() => void items.refetch()}>Reintentar</button>}
            </div>
          )}

          <div className="summary-grid">
            <SummaryCard
              icon={<Boxes size={18} />}
              iconTone="violet"
              label="Referencias activas"
              value={items.isPending ? '—' : inventory.length.toLocaleString('es-CO')}
              caption="Artículos registrados"
              trend={<><TrendingUp size={14} /> Catálogo</>}
            />
            <SummaryCard
              icon={<Package size={18} />}
              iconTone="mint"
              label="Unidades disponibles"
              value={items.isPending ? '—' : totalUnits.toLocaleString('es-CO')}
              caption="Unidades en inventario"
              trend={<><ArrowUp size={14} /> En existencias</>}
            />
            <SummaryCard
              icon={<AlertCircle size={18} />}
              iconTone="amber"
              label="Stock bajo"
              value={items.isPending ? '—' : lowStock.toLocaleString('es-CO')}
              caption="5 unidades o menos"
              trend={<><ArrowDown size={14} /> Requiere atención</>}
              warning
            />
            <SummaryCard
              icon={<CircleHelp size={18} />}
              iconTone="rose"
              label="Agotados"
              value={items.isPending ? '—' : outOfStock.toLocaleString('es-CO')}
              caption="Sin unidades disponibles"
              trend={<><ArrowDown size={14} /> Revisar reposición</>}
              warning={outOfStock > 0}
            />
          </div>

          <section className="inventory-panel">
            <div className="panel-heading">
              <div>
                <h2>Artículos</h2>
                <p>Administra y consulta tus productos.</p>
              </div>
              <button className="button button-secondary panel-add" onClick={() => { setNotice(''); setDialogItem(null) }}>
                <Plus size={16} /> Agregar artículo
              </button>
            </div>
            <div className="table-toolbar">
              <div className="result-count"><span className="result-dot" /> {inventory.length} {inventory.length === 1 ? 'artículo' : 'artículos'} en total</div>
              <label className="search-field">
                <Search size={16} />
                <input
                  aria-label="Buscar por nombre o SKU"
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Buscar artículo o SKU..."
                  value={search}
                />
                {search && <button aria-label="Limpiar búsqueda" className="search-clear" onClick={() => setSearch('')}><X size={14} /></button>}
              </label>
            </div>

            {items.isPending ? (
              <div className="table-loading"><LoaderCircle className="spin" size={23} /> Cargando inventario…</div>
            ) : items.isError ? (
              <div className="empty-state"><AlertCircle size={24} /><strong>No se pudo cargar el inventario</strong><span>Inténtalo de nuevo en unos momentos.</span></div>
            ) : filteredItems.length === 0 ? (
              <div className="empty-state">
                <div className="empty-icon"><Package size={24} /></div>
                <strong>{search ? 'No encontramos resultados' : 'Tu inventario está listo para empezar'}</strong>
                <span>{search ? 'Prueba con otro nombre o SKU.' : 'Agrega tu primer artículo para tener todo organizado.'}</span>
                {!search && <button className="button button-primary" onClick={() => setDialogItem(null)}><Plus size={16} /> Agregar artículo</button>}
              </div>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr><th>ARTÍCULO</th><th>SKU</th><th>ESTADO</th><th>UNIDADES</th><th>ÚLTIMA ACTUALIZACIÓN</th><th><span className="sr-only">Acciones</span></th></tr>
                  </thead>
                  <tbody>
                    {filteredItems.map((item) => (
                      <tr key={item.sku}>
                        <td>
                          <div className="product-cell">
                            <div className="product-icon"><Package size={17} /></div>
                            <strong>{item.name}</strong>
                          </div>
                        </td>
                        <td><span className="sku-code">{item.sku}</span></td>
                        <td><StockBadge quantity={item.quantity} /></td>
                        <td><span className="quantity">{item.quantity.toLocaleString('es-CO')} <span>uds.</span></span></td>
                        <td><span className="updated-date">{formatDate(item.updated_at)}</span></td>
                        <td>
                          <div className="row-actions">
                            <button
                              aria-label={`Editar ${item.name}`}
                              className="icon-button"
                              onClick={() => { setNotice(''); setDialogItem(item) }}
                              title="Editar"
                            ><Pencil size={16} /></button>
                            <button
                              aria-label={`Eliminar ${item.name}`}
                              className="icon-button delete-action"
                              onClick={() => { setNotice(''); setDeleteItem(item) }}
                              title="Eliminar"
                            ><Trash2 size={16} /></button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <footer className="panel-footer">
              <span>Mostrando <strong>{filteredItems.length}</strong> de <strong>{inventory.length}</strong> artículos</span>
              <span className="secure-label"><ShieldCheck size={14} /> Sincronizado con el servicio</span>
            </footer>
          </section>
          <footer className="app-footer"><span>Micro Inventory <span>·</span> Gestión inteligente</span><span>Hecho para mantener todo en orden <Sparkles size={13} /></span></footer>
        </section>
      </main>

      {dialogItem !== undefined && (
        <ItemDialog
          item={dialogItem}
          isSaving={saveItem.isPending}
          error={saveItem.error instanceof Error ? saveItem.error.message : undefined}
          onClose={() => { setDialogItem(undefined); saveItem.reset() }}
          onSave={submitItem}
        />
      )}
      {deleteItem && (
        <ConfirmDialog
          item={deleteItem}
          isDeleting={removeItem.isPending}
          error={removeItem.error instanceof Error ? removeItem.error.message : undefined}
          onClose={() => { setDeleteItem(null); removeItem.reset() }}
          onConfirm={() => removeItem.mutate(deleteItem.sku)}
        />
      )}
    </div>
  )
}

function SummaryCard({
  icon,
  iconTone,
  label,
  value,
  caption,
  trend,
  warning = false,
}: {
  icon: ReactNode
  iconTone: string
  label: string
  value: string
  caption: string
  trend: ReactNode
  warning?: boolean
}) {
  return (
    <article className="summary-card">
      <div className="summary-top"><div className={`summary-icon ${iconTone}`}>{icon}</div><span className={`summary-trend ${warning ? 'trend-warning' : ''}`}>{trend}</span></div>
      <div className="summary-label">{label}</div>
      <div className="summary-value">{value}</div>
      <div className="summary-caption">{caption}</div>
    </article>
  )
}

function StockBadge({ quantity }: { quantity: number }) {
  if (quantity === 0) return <span className="stock-badge stock-empty"><span /> Agotado</span>
  if (quantity <= 5) return <span className="stock-badge stock-low"><span /> Stock bajo</span>
  return <span className="stock-badge stock-ok"><span /> Disponible</span>
}

function ItemDialog({
  item,
  isSaving,
  error,
  onClose,
  onSave,
}: {
  item: InventoryItem | null
  isSaving: boolean
  error?: string
  onClose: () => void
  onSave: (item: InventoryItem) => void
}) {
  const [sku, setSku] = useState(item?.sku ?? '')
  const [name, setName] = useState(item?.name ?? '')
  const [quantity, setQuantity] = useState(item?.quantity.toString() ?? '0')

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    onSave({ sku: sku.trim(), name: name.trim(), quantity: Number(quantity) })
  }

  return (
    <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget && !isSaving) onClose() }}>
      <section aria-labelledby="item-dialog-title" aria-modal="true" className="dialog-card" role="dialog">
        <div className="dialog-header">
          <div className="dialog-icon"><Package size={20} /></div>
          <button aria-label="Cerrar" className="icon-button dialog-close" onClick={onClose}><X size={18} /></button>
          <div className="eyebrow">{item ? 'EDITAR INVENTARIO' : 'NUEVO REGISTRO'}</div>
          <h2 id="item-dialog-title">{item ? 'Actualizar artículo' : 'Agregar artículo'}</h2>
          <p>{item ? 'Modifica los datos de este artículo.' : 'Completa los datos para agregarlo a tu inventario.'}</p>
        </div>
        <form className="dialog-form" onSubmit={submit}>
          <label htmlFor="item-name">Nombre del artículo</label>
          <input autoFocus id="item-name" maxLength={120} onChange={(event) => setName(event.target.value)} placeholder="Ej. Teclado inalámbrico" required value={name} />
          <label htmlFor="item-sku">SKU <span>Código único</span></label>
          <input
            disabled={Boolean(item)}
            id="item-sku"
            maxLength={80}
            onChange={(event) => setSku(event.target.value)}
            placeholder="Ej. TEC-001"
            required
            value={sku}
          />
          <label htmlFor="item-quantity">Unidades disponibles</label>
          <input id="item-quantity" min="0" onChange={(event) => setQuantity(event.target.value)} required step="1" type="number" value={quantity} />
          {error && <p className="form-error" role="alert">{error}</p>}
          <div className="dialog-actions">
            <button className="button button-secondary" disabled={isSaving} onClick={onClose} type="button">Cancelar</button>
            <button className="button button-primary" disabled={isSaving} type="submit">
              {isSaving && <LoaderCircle className="spin" size={16} />}
              {isSaving ? 'Guardando…' : item ? 'Guardar cambios' : 'Agregar artículo'}
            </button>
          </div>
        </form>
      </section>
    </div>
  )
}

function ConfirmDialog({
  item,
  isDeleting,
  error,
  onClose,
  onConfirm,
}: {
  item: InventoryItem
  isDeleting: boolean
  error?: string
  onClose: () => void
  onConfirm: () => void
}) {
  return (
    <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget && !isDeleting) onClose() }}>
      <section aria-labelledby="delete-dialog-title" aria-modal="true" className="dialog-card confirm-card" role="dialog">
        <div className="confirm-icon"><Trash2 size={21} /></div>
        <button aria-label="Cerrar" className="icon-button dialog-close" onClick={onClose}><X size={18} /></button>
        <h2 id="delete-dialog-title">¿Eliminar este artículo?</h2>
        <p><strong>{item.name}</strong> ({item.sku}) se quitará del inventario. Esta acción no se puede deshacer.</p>
        {error && <p className="form-error" role="alert">{error}</p>}
        <div className="dialog-actions">
          <button className="button button-secondary" disabled={isDeleting} onClick={onClose}>Cancelar</button>
          <button className="button button-danger" disabled={isDeleting} onClick={onConfirm}>
            {isDeleting && <LoaderCircle className="spin" size={16} />}
            {isDeleting ? 'Eliminando…' : 'Sí, eliminar'}
          </button>
        </div>
      </section>
    </div>
  )
}

function Brand() {
  return (
    <a className="brand" href="/" aria-label="Micro Inventory, inicio">
      <span className="brand-mark"><span /><span /><span /><span /></span>
      <span className="brand-name">micro<span>inventory</span></span>
    </a>
  )
}

function formatDate(value: string | undefined) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('es-CO', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  }).format(date)
}

export default App
