(function () {
  const storageKey = 'daxi-cloud-leads-v1';
  const adminTokenKey = 'daxi-cloud-admin-token';
  const defaultApiBase = window.location.protocol.startsWith('http') ? window.location.origin : 'http://127.0.0.1:8088';
  const apiBase = (window.DAXI_API_BASE || defaultApiBase).replace(/\/$/, '');

  const scenarioPaths = {
    'Model API Access': {
      title: 'API Trial',
      detail: 'Best for developer integration, model comparison, and early API testing.',
    },
    'Short Drama Generation': {
      title: 'PoC Project',
      detail: 'Best for validating script, storyboard, subtitle, cover, and ad-material workflows.',
    },
    'E-commerce Short Video': {
      title: 'Campaign PoC',
      detail: 'Best for SKU-based video variants, marketplace captions, and social commerce testing.',
    },
    'Enterprise / Campus Agent': {
      title: 'Agent Solution Workshop',
      detail: 'Best for knowledge base, campus service, support, education, or operations automation.',
    },
    'AI Compute Requirement': {
      title: 'Custom Compute Quote',
      detail: 'Best for GPU cloud, bare metal, dedicated cluster, or private deployment.',
    },
  };

  const leadStatuses = ['New', 'Contacted', 'Qualified', 'Converted', 'Closed'];
  let currentLeadItems = [];

  const sampleLeads = [
    {
      public_id: 'DX-1007',
      scenario: 'Short Drama Generation',
      company: 'ASEAN Media Studio',
      country: 'Malaysia',
      usage_profile: 'Campaign production',
      budget: 'PoC Project',
      email: 'producer@example.com',
      status: 'New',
      created_at: '2026-06-09 15:40',
    },
    {
      public_id: 'DX-1008',
      scenario: 'E-commerce Short Video',
      company: 'Crossborder Beauty Seller',
      country: 'Singapore',
      usage_profile: 'Campaign production',
      budget: 'Monthly subscription',
      email: 'growth@example.com',
      status: 'Reviewing',
      created_at: '2026-06-09 16:05',
    },
    {
      public_id: 'DX-1009',
      scenario: 'AI Compute Requirement',
      company: 'University Research Lab',
      country: 'Malaysia',
      usage_profile: 'GPU / private deployment',
      budget: 'Custom compute quote',
      email: 'lab@example.edu',
      status: 'Quoting',
      created_at: '2026-06-09 16:22',
    },
  ];

  function getLeads() {
    try {
      return JSON.parse(localStorage.getItem(storageKey) || '[]');
    } catch {
      return [];
    }
  }

  function setLeads(leads) {
    localStorage.setItem(storageKey, JSON.stringify(leads));
  }

  function makeLeadId() {
    return `DX-${Math.floor(1000 + Math.random() * 9000)}`;
  }

  function formatDate(value) {
    const date = value ? new Date(value) : new Date();
    if (Number.isNaN(date.getTime())) return value || '-';
    const pad = (part) => String(part).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
  }

  function adminToken() {
    return localStorage.getItem(adminTokenKey) || 'dev-admin-token';
  }

  function setAdminToken(value) {
    localStorage.setItem(adminTokenKey, value || 'dev-admin-token');
  }

  function leadID(lead) {
    return lead.public_id || lead.id || makeLeadId();
  }

  function cacheLead(lead) {
    setLeads([lead, ...getLeads()]);
  }

  function leadFromForm(data) {
    return {
      source: 'get-started',
      scenario: data.get('scenario'),
      company: data.get('company'),
      country: data.get('country'),
      usage_profile: data.get('usage'),
      budget: data.get('budget'),
      email: data.get('email'),
      phone: data.get('phone'),
      notes: data.get('notes'),
      status: 'New',
      created_at: new Date().toISOString(),
    };
  }

  async function apiJSON(path, options = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    };
    if (options.admin) {
      headers.Authorization = `Bearer ${adminToken()}`;
    }
    const response = await fetch(`${apiBase}${path}`, {
      ...options,
      headers,
    });
    const text = await response.text();
    const payload = text ? JSON.parse(text) : {};
    if (!response.ok) {
      throw new Error(payload.error || `Request failed: ${response.status}`);
    }
    return payload;
  }

  async function submitPublicLead(path, payload, fallbackStatus) {
    try {
      const saved = await apiJSON(path, {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      return { lead: saved.lead || saved, online: true };
    } catch (error) {
      const offlineLead = {
        ...payload,
        public_id: makeLeadId(),
        status: 'Local draft',
        created_at: new Date().toISOString(),
        offline_error: error.message,
      };
      cacheLead(offlineLead);
      return { lead: offlineLead, online: false, fallbackStatus };
    }
  }

  function setupScenarioPicker() {
    const picker = document.querySelector('[data-scenario-picker]');
    const input = document.querySelector('[data-scenario-input]');
    const recommendation = document.querySelector('[data-recommendation-card]');
    if (!picker || !input) return;

    picker.addEventListener('click', (event) => {
      const card = event.target.closest('[data-scenario]');
      if (!card) return;
      picker.querySelectorAll('[data-scenario]').forEach((item) => item.classList.remove('selected'));
      card.classList.add('selected');
      input.value = card.dataset.scenario;
      const scenarioLabel = document.querySelector('[data-selected-scenario]');
      if (scenarioLabel) scenarioLabel.textContent = card.dataset.scenario;

      if (recommendation) {
        const path = scenarioPaths[card.dataset.scenario] || scenarioPaths['Model API Access'];
        recommendation.innerHTML = `<strong>${path.title}</strong><span>${path.detail}</span>`;
      }
    });
  }

  function setupLeadForm() {
    const form = document.querySelector('[data-lead-form]');
    const status = document.querySelector('[data-form-status]');
    if (!form) return;
    const scenarioInput = document.querySelector('[data-scenario-input]');
    const scenarioLabel = document.querySelector('[data-selected-scenario]');
    const params = new URLSearchParams(window.location.search);
    const requestedScenario = params.get('scenario');
    if (requestedScenario && scenarioInput) {
      scenarioInput.value = requestedScenario;
      if (scenarioLabel) scenarioLabel.textContent = requestedScenario;
    } else if (scenarioInput && scenarioLabel) {
      scenarioLabel.textContent = scenarioInput.value;
    }

    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const payload = leadFromForm(new FormData(form));
      const currentScenario = payload.scenario;
      if (status) status.textContent = 'Submitting requirement...';

      const result = await submitPublicLead('/public/leads', payload);
      form.reset();
      const selected = document.querySelector('[data-scenario-picker] .selected');
      if (selected) {
        document.querySelector('[data-scenario-input]').value = selected.dataset.scenario;
      } else if (scenarioInput && currentScenario) {
        scenarioInput.value = currentScenario;
      }
      if (scenarioLabel && scenarioInput) scenarioLabel.textContent = scenarioInput.value;
      if (status) {
        const suffix = result.online ? 'It is now visible in the Admin Console.' : 'Backend is offline, so it was saved as a local draft.';
        status.textContent = `Requirement ${leadID(result.lead)} submitted. ${suffix}`;
      }
    });
  }

  async function fetchAdminData() {
    const [dashboard, leads, customers, keys, usage] = await Promise.all([
      apiJSON('/admin/dashboard', { admin: true }),
      apiJSON('/admin/leads?limit=100', { admin: true }),
      apiJSON('/admin/customers?limit=100', { admin: true }),
      apiJSON('/admin/api-keys?limit=100', { admin: true }),
      apiJSON('/admin/usage', { admin: true }),
    ]);
    const [plans, orders, recharges, usageRecords] = await Promise.all([
      apiJSON('/admin/token-plans', { admin: true }),
      apiJSON('/admin/token-orders?limit=100', { admin: true }),
      apiJSON('/admin/recharges?limit=100', { admin: true }),
      apiJSON('/admin/usage-records?limit=100', { admin: true }),
    ]);
    const [modelRoutes, videoTasks, payments, adminUsers, auditLogs] = await Promise.all([
      apiJSON('/admin/model-routes', { admin: true }),
      apiJSON('/admin/video-tasks?limit=100', { admin: true }),
      apiJSON('/admin/payments?limit=100', { admin: true }),
      apiJSON('/admin/users?limit=100', { admin: true }),
      apiJSON('/admin/audit-logs?limit=100', { admin: true }),
    ]);
    return {
      dashboard,
      leads: leads.items || [],
      customers: customers.items || [],
      keys: keys.items || [],
      usage,
      plans: plans.items || [],
      orders: orders.items || [],
      recharges: recharges.items || [],
      usageRecords: usageRecords.items || [],
      modelRoutes: modelRoutes.items || [],
      videoTasks: videoTasks.items || [],
      payments: payments.items || [],
      adminUsers: adminUsers.items || [],
      auditLogs: auditLogs.items || [],
    };
  }

  function renderKPIs(data) {
    const calls = document.querySelector('[data-api-call-count]');
    const videoTasks = document.querySelector('[data-video-task-count]');
    const leadCount = document.querySelector('[data-lead-count]');
    const upstream = document.querySelector('[data-upstream-status]');
    const customerCount = document.querySelector('[data-customer-count]');
    const keyCount = document.querySelector('[data-api-key-count]');
    const usageTotal = document.querySelector('[data-usage-total]');

    if (!data) return;
    if (calls) calls.textContent = String(data.usage.request_count || data.dashboard.usage?.request_count || 0);
    if (videoTasks) videoTasks.textContent = String(data.dashboard.video_task_count || data.videoTasks?.length || 0);
    if (leadCount) leadCount.textContent = String(data.dashboard.lead_count || data.leads.length);
    if (customerCount) customerCount.textContent = String(data.dashboard.customer_count || data.customers.length);
    if (keyCount) keyCount.textContent = String(data.dashboard.api_key_count || data.keys.length);
    if (usageTotal) usageTotal.textContent = String(data.usage.total_tokens || 0);
    if (upstream) {
      upstream.textContent = data.dashboard.upstream?.emergency_disabled ? 'Disabled' : 'Active';
    }
  }

  function renderLeadTableFromItems(items) {
    const table = document.querySelector('[data-lead-table]');
    if (!table) return;
    currentLeadItems = items;

    if (!items.length) {
      table.innerHTML = `
        <div class="empty-state">
          <strong>No submitted requirements yet.</strong>
          <span>Use Get Started to submit a requirement, or load sample leads for demo.</span>
        </div>
      `;
      return;
    }

    table.innerHTML = `
      <div class="lead-row lead-row-head">
        <span>ID</span><span>Scenario</span><span>Company</span><span>Contact</span><span>Status</span><span>Created</span><span>Action</span>
      </div>
      ${items.map((lead) => `
        <div class="lead-row" data-lead-id="${leadID(lead)}">
          <span>${leadID(lead)}</span>
          <span>${lead.scenario || '-'}</span>
          <span>${lead.company || '-'}</span>
          <span>${lead.email || lead.phone || '-'}</span>
          <span><mark>${lead.status || 'New'}</mark></span>
          <span>${formatDate(lead.created_at || lead.createdAt)}</span>
          <span class="lead-status-control">
            <select data-lead-status>
              ${leadStatuses.map((status) => `<option value="${status}" ${status === (lead.status || 'New') ? 'selected' : ''}>${status}</option>`).join('')}
            </select>
            <button type="button" data-lead-status-save>Update</button>
            <button type="button" class="button-secondary" data-lead-detail-open>Details</button>
          </span>
        </div>
      `).join('')}
    `;
  }

  function renderLeadDetail(lead) {
    const panel = document.querySelector('[data-lead-detail]');
    if (!panel) return;
    if (!lead) {
      panel.innerHTML = `
        <div class="empty-state">
          <strong>Select a lead to review details.</strong>
          <span>Contact information, scenario notes, and handoff guidance will appear here.</span>
        </div>
      `;
      return;
    }
    const publicID = leadID(lead);
    const contact = [lead.contact_name, lead.email, lead.phone].filter(Boolean).join(' / ') || '-';
    panel.innerHTML = `
      <div class="lead-detail-card">
        <div class="lead-detail-head">
          <div>
            <p class="eyebrow">LEAD DETAIL</p>
            <h3>${lead.company || 'Unqualified Lead'}</h3>
            <span>${publicID} · ${lead.scenario || 'Scenario pending'}</span>
          </div>
          <mark>${lead.status || 'New'}</mark>
        </div>
        <div class="lead-detail-grid">
          <div><span>Contact</span><strong>${contact}</strong></div>
          <div><span>Country</span><strong>${lead.country || '-'}</strong></div>
          <div><span>Usage Profile</span><strong>${lead.usage_profile || '-'}</strong></div>
          <div><span>Budget / Package</span><strong>${lead.budget || '-'}</strong></div>
          <div><span>Source</span><strong>${lead.source || 'public form'}</strong></div>
          <div><span>Created</span><strong>${formatDate(lead.created_at || lead.createdAt)}</strong></div>
        </div>
        <div class="lead-detail-notes">
          <span>Notes</span>
          <p>${lead.notes || 'No additional notes provided.'}</p>
        </div>
        <div class="lead-handoff">
          <strong>Recommended handoff</strong>
          <span>Confirm contact and usage scope, then send the qualified lead to platform operations for customer record, API key, token package, and recharge setup.</span>
        </div>
      </div>
    `;
  }

  async function updateLeadStatus(publicID, status) {
    return apiJSON(`/admin/leads/${encodeURIComponent(publicID)}/status`, {
      method: 'PATCH',
      admin: true,
      body: JSON.stringify({ status }),
    });
  }

  function updateLocalLeadStatus(publicID, status) {
    const leads = getLeads();
    const next = leads.map((lead) => (
      leadID(lead) === publicID ? { ...lead, status, updated_at: new Date().toISOString() } : lead
    ));
    setLeads(next);
  }

  function renderCustomerTable(customers) {
    const table = document.querySelector('[data-customer-table]');
    const select = document.querySelector('[data-api-key-customer]');
    const rechargeSelect = document.querySelector('[data-recharge-customer]');
    const orderCustomerSelect = document.querySelector('[data-order-customer]');
    const customerDetailSelect = document.querySelector('[data-customer-detail-select]');
    if (!table) return;

    [select, rechargeSelect, orderCustomerSelect, customerDetailSelect].forEach((item) => {
      if (item) item.innerHTML = customers.map((customer) => `<option value="${customer.public_id}">${customer.company} (${customer.public_id})</option>`).join('');
    });

    if (!customers.length) {
      table.innerHTML = '<div class="empty-state"><strong>No customers yet.</strong><span>Create a customer to issue downstream API keys.</span></div>';
      return;
    }

    table.innerHTML = `
      <div class="ops-row ops-row-head"><span>Customer</span><span>Country</span><span>Balance</span><span>Status</span></div>
      ${customers.map((customer) => `
        <div class="ops-row">
          <span>${customer.company}<small>${customer.public_id}</small></span>
          <span>${customer.country || '-'}</span>
          <span>${customer.balance_tokens || 0}</span>
          <span><mark>${customer.status || 'Active'}</mark></span>
        </div>
      `).join('')}
    `;
  }

  function renderKeyTable(keys) {
    const table = document.querySelector('[data-key-table]');
    if (!table) return;

    if (!keys.length) {
      table.innerHTML = '<div class="empty-state"><strong>No API keys yet.</strong><span>Select a customer and issue the first downstream key.</span></div>';
      return;
    }

    table.innerHTML = `
      <div class="ops-row ops-row-head"><span>Key</span><span>Customer ID</span><span>Scope</span><span>Status</span></div>
      ${keys.map((key) => `
        <div class="ops-row">
          <span>${key.name}<small>${key.key_prefix}...</small></span>
          <span>${key.customer_id}</span>
          <span>${key.scopes || '-'}</span>
          <span><mark>${key.status || 'Active'}</mark></span>
        </div>
      `).join('')}
    `;
  }

  function renderPlanSelect(plans) {
    const select = document.querySelector('[data-order-plan]');
    if (!select) return;
    select.innerHTML = plans.map((plan) => `<option value="${plan.public_id}">${plan.name} - ${plan.tokens} tokens</option>`).join('');
  }

  function renderOrderTable(orders) {
    const table = document.querySelector('[data-order-table]');
    if (!table) return;
    if (!orders.length) {
      table.innerHTML = '<div class="empty-state"><strong>No token orders yet.</strong><span>Create an order from a customer and plan.</span></div>';
      return;
    }
    table.innerHTML = `
      <div class="ops-row ops-row-head order-row"><span>Order</span><span>Plan</span><span>Tokens</span><span>Status</span><span>Created</span></div>
      ${orders.map((order) => `
        <div class="ops-row order-row">
          <span>${order.public_id}<small>Customer #${order.customer_id}</small></span>
          <span>${order.plan_name}</span>
          <span>${order.tokens}</span>
          <span><mark>${order.status}</mark></span>
          <span>${formatDate(order.created_at)}</span>
        </div>
      `).join('')}
    `;
  }

  function renderRechargeTable(recharges) {
    const table = document.querySelector('[data-recharge-table]');
    if (!table) return;
    if (!recharges.length) {
      table.innerHTML = '<div class="empty-state"><strong>No recharge records yet.</strong><span>Manual top-ups will appear here.</span></div>';
      return;
    }
    table.innerHTML = `
      <div class="ops-row ops-row-head recharge-row"><span>Recharge</span><span>Customer</span><span>Tokens</span><span>Balance</span><span>Source</span></div>
      ${recharges.map((item) => `
        <div class="ops-row recharge-row">
          <span>${item.public_id}<small>${formatDate(item.created_at)}</small></span>
          <span>${item.customer_public_id}</span>
          <span>${item.tokens}</span>
          <span>${item.balance_after}</span>
          <span><mark>${item.source}</mark></span>
        </div>
      `).join('')}
    `;
  }

  function renderUsageRecordTable(records) {
    const table = document.querySelector('[data-usage-record-table]');
    if (!table) return;
    if (!records.length) {
      table.innerHTML = '<div class="empty-state"><strong>No usage records yet.</strong><span>Completed model proxy calls with usage will appear here.</span></div>';
      return;
    }
    table.innerHTML = `
      <div class="ops-row ops-row-head usage-row"><span>Request</span><span>Customer</span><span>Model</span><span>Tokens</span><span>Status</span><span>Created</span></div>
      ${records.map((record) => `
        <div class="ops-row usage-row">
          <span>${record.request_id}</span>
          <span>${record.customer_company}<small>${record.customer_public_id}</small></span>
          <span>${record.model || '-'}</span>
          <span>${record.total_tokens}</span>
          <span><mark>${record.status_code}</mark></span>
          <span>${formatDate(record.created_at)}</span>
        </div>
      `).join('')}
    `;
  }

  function renderModelRouteTable(routes) {
    const table = document.querySelector('[data-model-route-table]');
    if (!table) return;
    if (!routes.length) {
      table.innerHTML = '<div class="empty-state"><strong>No model routes yet.</strong><span>Create scenario routes before opening downstream access.</span></div>';
      return;
    }
    table.innerHTML = `
      <div class="ops-row ops-row-head route-row"><span>Scenario</span><span>Primary model</span><span>Fallbacks</span><span>Limit</span><span>Status</span></div>
      ${routes.map((route) => `
        <div class="ops-row route-row">
          <span>${route.scenario}<small>${route.public_id}</small></span>
          <span>${route.primary_model}</span>
          <span>${route.fallback_models || '-'}</span>
          <span>${route.max_tokens_per_request || '-'}</span>
          <span><mark>${route.status}</mark></span>
        </div>
      `).join('')}
    `;
  }

  function renderVideoTaskTable(tasks) {
    const table = document.querySelector('[data-video-task-table]');
    if (!table) return;
    if (!tasks.length) {
      table.innerHTML = '<div class="empty-state"><strong>No video tasks yet.</strong><span>Short drama and e-commerce video jobs will appear here.</span></div>';
      return;
    }
    table.innerHTML = `
      <div class="ops-row ops-row-head video-row"><span>Task</span><span>Type</span><span>Progress</span><span>Status</span><span>Created</span></div>
      ${tasks.map((task) => `
        <div class="ops-row video-row">
          <span>${task.title || task.public_id}<small>${task.public_id}</small></span>
          <span>${task.task_type}</span>
          <span>${task.progress || 0}%</span>
          <span><mark>${task.status}</mark></span>
          <span>${formatDate(task.created_at)}</span>
        </div>
      `).join('')}
    `;
  }

  function renderPaymentTable(payments) {
    const table = document.querySelector('[data-payment-table]');
    if (!table) return;
    if (!payments.length) {
      table.innerHTML = '<div class="empty-state"><strong>No payment records yet.</strong><span>Offline transfers, invoices, and payment confirmations will appear here.</span></div>';
      return;
    }
    table.innerHTML = `
      <div class="ops-row ops-row-head payment-row"><span>Payment</span><span>Customer</span><span>Amount</span><span>Status</span><span>Reference</span></div>
      ${payments.map((payment) => `
        <div class="ops-row payment-row">
          <span>${payment.public_id}<small>${formatDate(payment.created_at)}</small></span>
          <span>${payment.customer_public_id || '-'}</span>
          <span>${payment.currency} ${(payment.amount_cents / 100).toFixed(2)}</span>
          <span><mark>${payment.status}</mark></span>
          <span>${payment.reference_id || '-'}</span>
        </div>
      `).join('')}
    `;
  }

  function renderAdminUserTable(users) {
    const table = document.querySelector('[data-admin-user-table]');
    if (!table) return;
    if (!users.length) {
      table.innerHTML = '<div class="empty-state"><strong>No admin users yet.</strong><span>The default owner account will appear after backend startup.</span></div>';
      return;
    }
    table.innerHTML = `
      <div class="ops-row ops-row-head admin-row"><span>User</span><span>Role</span><span>Status</span><span>Last login</span></div>
      ${users.map((user) => `
        <div class="ops-row admin-row">
          <span>${user.username}<small>${user.public_id}</small></span>
          <span>${user.role}</span>
          <span><mark>${user.status}</mark></span>
          <span>${formatDate(user.last_login_at)}</span>
        </div>
      `).join('')}
    `;
  }

  async function renderCustomerDetail(publicID) {
    const panel = document.querySelector('[data-customer-detail]');
    if (!panel || !publicID) return;
    panel.innerHTML = '<div class="empty-state"><strong>Loading customer detail...</strong></div>';
    try {
      const detail = await apiJSON(`/admin/customers/${publicID}`, { admin: true });
      const customer = detail.customer;
      panel.innerHTML = `
        <div class="detail-grid">
          <div><span>Customer</span><strong>${customer.company}</strong><small>${customer.public_id}</small></div>
          <div><span>Balance</span><strong>${customer.balance_tokens}</strong><small>available tokens</small></div>
          <div><span>API Keys</span><strong>${detail.api_keys.length}</strong><small>issued keys</small></div>
          <div><span>Total Usage</span><strong>${detail.usage.total_tokens}</strong><small>${detail.usage.request_count} requests</small></div>
        </div>
      `;
    } catch (error) {
      panel.innerHTML = `<div class="empty-state"><strong>${error.message}</strong></div>`;
    }
  }

  async function renderAdminConsole() {
    const table = document.querySelector('[data-lead-table]');
    if (!table) return;

    try {
      const data = await fetchAdminData();
      renderKPIs(data);
      renderLeadTableFromItems(data.leads);
      renderCustomerTable(data.customers);
      renderKeyTable(data.keys);
      renderPlanSelect(data.plans);
      renderOrderTable(data.orders);
      renderRechargeTable(data.recharges);
      renderUsageRecordTable(data.usageRecords);
      renderModelRouteTable(data.modelRoutes);
      renderVideoTaskTable(data.videoTasks);
      renderPaymentTable(data.payments);
      renderAdminUserTable(data.adminUsers);
      const detailSelect = document.querySelector('[data-customer-detail-select]');
      if (detailSelect && detailSelect.value) renderCustomerDetail(detailSelect.value);
      const status = document.querySelector('[data-console-status]');
      if (status) status.textContent = `Connected to ${apiBase}`;
    } catch (error) {
      const localLeads = getLeads();
      renderLeadTableFromItems(localLeads);
      renderKPIs({ dashboard: { lead_count: localLeads.length, customer_count: 0, api_key_count: 0, upstream: {} }, leads: localLeads, customers: [], keys: [], usage: {} });
      renderCustomerTable([]);
      renderKeyTable([]);
      renderPlanSelect([]);
      renderOrderTable([]);
      renderRechargeTable([]);
      renderUsageRecordTable([]);
      renderModelRouteTable([]);
      renderVideoTaskTable([]);
      renderPaymentTable([]);
      renderAdminUserTable([]);
      const status = document.querySelector('[data-console-status]');
      if (status) status.textContent = `Backend unavailable: ${error.message}. Showing local drafts.`;
    }
  }

  function setupSampleLeads() {
    const button = document.querySelector('[data-seed-leads]');
    if (!button) return;
    button.addEventListener('click', () => {
      setLeads([...sampleLeads, ...getLeads()]);
      renderAdminConsole();
    });
  }

  function setupLeadStatusActions() {
    const table = document.querySelector('[data-lead-table]');
    if (!table) return;
    table.addEventListener('click', async (event) => {
      const detailButton = event.target.closest('[data-lead-detail-open]');
      if (detailButton) {
        const row = detailButton.closest('[data-lead-id]');
        const lead = currentLeadItems.find((item) => leadID(item) === row?.dataset.leadId);
        renderLeadDetail(lead);
        return;
      }

      const button = event.target.closest('[data-lead-status-save]');
      if (!button) return;
      const row = button.closest('[data-lead-id]');
      const select = row?.querySelector('[data-lead-status]');
      const publicID = row?.dataset.leadId;
      const status = select?.value;
      if (!row || !publicID || !status) return;

      const original = button.textContent;
      button.disabled = true;
      button.textContent = 'Saving...';
      try {
        await updateLeadStatus(publicID, status);
        button.textContent = 'Saved';
        renderAdminConsole();
      } catch (error) {
        if (publicID.startsWith('DX-')) {
          updateLocalLeadStatus(publicID, status);
          button.textContent = 'Saved local';
          renderAdminConsole();
        } else {
          button.textContent = error.message || 'Failed';
        }
      } finally {
        window.setTimeout(() => {
          button.disabled = false;
          button.textContent = original;
        }, 1200);
      }
    });
  }

  function setupAdminTokenForm() {
    const form = document.querySelector('[data-admin-token-form]');
    if (!form) return;
    const input = form.querySelector('input');
    input.value = adminToken();
    form.addEventListener('submit', (event) => {
      event.preventDefault();
      setAdminToken(input.value.trim());
      renderAdminConsole();
    });
  }

  function setupAdminLoginForm() {
    const form = document.querySelector('[data-admin-login-form]');
    const status = document.querySelector('[data-admin-login-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const data = new FormData(form);
      if (status) status.textContent = 'Signing in...';
      try {
        const response = await apiJSON('/admin/sessions', {
          method: 'POST',
          body: JSON.stringify({
            username: data.get('username'),
            password: data.get('password'),
          }),
        });
        setAdminToken(response.session_token);
        if (status) status.textContent = 'Signed in with admin session.';
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupCustomerForm() {
    const form = document.querySelector('[data-customer-form]');
    const status = document.querySelector('[data-customer-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Creating customer...';
      try {
        await apiJSON('/admin/customers', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            company: data.get('company'),
            country: data.get('country'),
            email: data.get('email'),
            balance_tokens: Number(data.get('balance') || 0),
          }),
        });
        form.reset();
        if (status) status.textContent = 'Customer created.';
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupAPIKeyForm() {
    const form = document.querySelector('[data-api-key-form]');
    const status = document.querySelector('[data-api-key-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Issuing API key...';
      try {
        const response = await apiJSON('/admin/api-keys', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            customer_public_id: data.get('customer_public_id'),
            name: data.get('name'),
            scopes: data.get('scopes'),
          }),
        });
        form.reset();
        if (status) status.textContent = `API key issued. Secret: ${response.secret}`;
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupOrderForm() {
    const form = document.querySelector('[data-order-form]');
    const status = document.querySelector('[data-order-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Creating token order...';
      try {
        const order = await apiJSON('/admin/token-orders', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            customer_public_id: data.get('customer_public_id'),
            plan_public_id: data.get('plan_public_id'),
            notes: data.get('notes'),
          }),
        });
        if (status) status.textContent = `Order ${order.public_id} created.`;
        form.reset();
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupRechargeForm() {
    const form = document.querySelector('[data-recharge-form]');
    const status = document.querySelector('[data-recharge-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Posting recharge...';
      try {
        const recharge = await apiJSON('/admin/recharges', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            customer_public_id: data.get('customer_public_id'),
            tokens: Number(data.get('tokens')),
            source: 'manual',
            reference_id: data.get('reference_id'),
            notes: data.get('notes'),
            operator: 'admin-console',
          }),
        });
        if (status) status.textContent = `Recharge ${recharge.public_id} posted. Balance: ${recharge.balance_after}.`;
        form.reset();
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupCustomerDetailPicker() {
    const select = document.querySelector('[data-customer-detail-select]');
    if (!select) return;
    select.addEventListener('change', () => renderCustomerDetail(select.value));
  }

  function setupModelRouteForm() {
    const form = document.querySelector('[data-model-route-form]');
    const status = document.querySelector('[data-model-route-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Saving model route...';
      try {
        await apiJSON('/admin/model-routes', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            scenario: data.get('scenario'),
            primary_model: data.get('primary_model'),
            fallback_models: data.get('fallback_models'),
            max_tokens_per_request: Number(data.get('max_tokens_per_request') || 0),
            status: data.get('status') || 'Active',
            notes: data.get('notes'),
          }),
        });
        form.reset();
        if (status) status.textContent = 'Model route saved.';
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupVideoTaskForm() {
    const form = document.querySelector('[data-video-task-form]');
    const status = document.querySelector('[data-video-task-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Creating video task...';
      try {
        const task = await apiJSON('/admin/video-tasks', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            task_type: data.get('task_type'),
            customer_public_id: data.get('customer_public_id'),
            title: data.get('title'),
            prompt: data.get('prompt'),
            product_name: data.get('product_name'),
            language: data.get('language') || 'en',
          }),
        });
        form.reset();
        if (status) status.textContent = `Video task ${task.public_id} created.`;
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupPaymentForm() {
    const form = document.querySelector('[data-payment-form]');
    const status = document.querySelector('[data-payment-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Recording payment...';
      try {
        const payment = await apiJSON('/admin/payments', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            customer_public_id: data.get('customer_public_id'),
            order_public_id: data.get('order_public_id'),
            provider: data.get('provider') || 'manual',
            amount_cents: Math.round(Number(data.get('amount') || 0) * 100),
            currency: data.get('currency') || 'USD',
            status: data.get('status') || 'Pending',
            reference_id: data.get('reference_id'),
            notes: data.get('notes'),
          }),
        });
        form.reset();
        if (status) status.textContent = `Payment ${payment.public_id} recorded.`;
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupAdminUserForm() {
    const form = document.querySelector('[data-admin-user-form]');
    const status = document.querySelector('[data-admin-user-status]');
    if (!form) return;
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;
      const data = new FormData(form);
      if (status) status.textContent = 'Creating admin user...';
      try {
        const user = await apiJSON('/admin/users', {
          method: 'POST',
          admin: true,
          body: JSON.stringify({
            username: data.get('username'),
            password: data.get('password'),
            role: data.get('role') || 'Operator',
          }),
        });
        form.reset();
        if (status) status.textContent = `Admin user ${user.username} created.`;
        renderAdminConsole();
      } catch (error) {
        if (status) status.textContent = error.message;
      }
    });
  }

  function setupTokenPlans() {
    const status = document.querySelector('[data-token-status]');
    document.querySelectorAll('[data-token-plan]').forEach((button) => {
      button.addEventListener('click', async () => {
        const plan = button.dataset.tokenPlan;
        const payload = {
          scenario: 'Model API Access',
          company: 'Pending customer profile',
          country: 'To be confirmed',
          usage_profile: plan,
          budget: plan,
          email: 'To be collected',
          notes: `Token package request: ${plan}`,
        };
        if (status) status.textContent = 'Submitting token package request...';
        const result = await submitPublicLead('/public/token-package-requests', payload);
        if (status) {
          const suffix = result.online ? 'sent to backend.' : 'saved locally because backend is offline.';
          status.textContent = `${plan} request ${leadID(result.lead)} ${suffix}`;
        }
      });
    });
  }

  function setupComputeForm() {
    const form = document.querySelector('[data-compute-form]');
    const status = document.querySelector('[data-compute-status]');
    if (!form) return;

    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!form.reportValidity()) return;

      const data = new FormData(form);
      const payload = {
        company: data.get('company'),
        country: data.get('country'),
        resource_type: data.get('resourceType'),
        gpu: data.get('gpu'),
        quantity: data.get('quantity'),
        lease_period: data.get('lease'),
        budget: data.get('budget'),
        email: data.get('email'),
        notes: data.get('notes'),
      };
      if (status) status.textContent = 'Submitting compute inquiry...';
      const result = await submitPublicLead('/public/compute-inquiries', payload);
      form.reset();
      if (status) {
        const suffix = result.online ? 'It is now visible in the Admin Console.' : 'Backend is offline, so it was saved as a local draft.';
        status.textContent = `Compute inquiry ${leadID(result.lead)} submitted. ${suffix}`;
      }
    });
  }

  function setupAgentPacks() {
    const status = document.querySelector('[data-agent-status]');
    document.querySelectorAll('[data-agent-pack]').forEach((button) => {
      button.addEventListener('click', async () => {
        const pack = button.dataset.agentPack;
        const payload = {
          scenario: 'Enterprise / Campus Agent',
          company: 'Pending customer profile',
          country: 'To be confirmed',
          usage_profile: pack,
          budget: 'Agent PoC',
          email: 'To be collected',
          notes: `Agent pack request: ${pack}`,
        };
        if (status) status.textContent = 'Submitting agent request...';
        const result = await submitPublicLead('/public/agent-requests', payload);
        if (status) {
          const suffix = result.online ? 'sent to backend.' : 'saved locally because backend is offline.';
          status.textContent = `${pack} request ${leadID(result.lead)} ${suffix}`;
        }
      });
    });
  }

  function setupMobileMenu() {
    const button = document.querySelector('.mobile-menu-toggle');
    const header = document.querySelector('.site-header');
    if (!button || !header) return;
    button.addEventListener('click', () => {
      const isOpen = header.classList.toggle('menu-open');
      button.setAttribute('aria-expanded', String(isOpen));
    });
    header.querySelectorAll('.main-nav a').forEach((link) => {
      link.addEventListener('click', () => {
        header.classList.remove('menu-open');
        button.setAttribute('aria-expanded', 'false');
      });
    });
  }

  setupMobileMenu();
  setupScenarioPicker();
  setupLeadForm();
  setupSampleLeads();
  setupLeadStatusActions();
  setupAdminLoginForm();
  setupAdminTokenForm();
  setupCustomerForm();
  setupAPIKeyForm();
  setupOrderForm();
  setupRechargeForm();
  setupCustomerDetailPicker();
  setupModelRouteForm();
  setupVideoTaskForm();
  setupPaymentForm();
  setupAdminUserForm();
  setupTokenPlans();
  setupComputeForm();
  setupAgentPacks();
  renderAdminConsole();
})();
