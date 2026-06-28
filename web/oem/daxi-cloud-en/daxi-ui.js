
(function () {
  const iconPaths = {
    'globe-2': '<circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20"/>',
    box: '<path d="m21 8-9-5-9 5 9 5 9-5Z"/><path d="M3 8v8l9 5 9-5V8"/><path d="M12 13v8"/>',
    coins: '<ellipse cx="8" cy="7" rx="5" ry="3"/><path d="M3 7v5c0 1.7 2.2 3 5 3s5-1.3 5-3V7"/><path d="M11 14c.8.7 2.2 1 4 1 2.8 0 5-1.3 5-3V9"/><ellipse cx="15" cy="9" rx="5" ry="3"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8"/>',
    blocks: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
    'check-circle-2': '<circle cx="12" cy="12" r="10"/><path d="m8 12 3 3 5-6"/>',
    clapperboard: '<path d="M4 11h16v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-8Z"/><path d="m4 11 2-7 14 4-1 3M8 5l3 5M14 7l3 5"/>',
    'shopping-cart': '<circle cx="9" cy="20" r="1"/><circle cx="18" cy="20" r="1"/><path d="M2 3h3l3 12h10l3-8H6"/>',
    headphones: '<path d="M3 18v-6a9 9 0 0 1 18 0v6"/><path d="M21 19a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3v5ZM3 19a2 2 0 0 0 2 2h1a2 2 0 0 0 2-2v-3a2 2 0 0 0-2-2H3v5Z"/>',
    'book-open': '<path d="M2 4h7a3 3 0 0 1 3 3v14a3 3 0 0 0-3-3H2V4ZM22 4h-7a3 3 0 0 0-3 3v14a3 3 0 0 1 3-3h7V4Z"/>',
    gauge: '<path d="M12 14l4-4"/><path d="M3.3 19a10 10 0 1 1 17.4 0"/><path d="M12 21a3 3 0 0 0 3-3H9a3 3 0 0 0 3 3Z"/>',
    'wallet-cards': '<rect x="3" y="6" width="18" height="14" rx="2"/><path d="M3 10h18M7 14h5"/>',
    'bar-chart-3': '<path d="M3 3v18h18"/><path d="M7 16v-5M12 16V7M17 16v-8"/>',
    'shield-check': '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z"/><path d="m9 12 2 2 4-5"/>',
    sparkles: '<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8L12 3Z"/><path d="M5 3v4M3 5h4M19 17v4M17 19h4"/>',
    check: '<path d="m5 12 4 4L19 6"/>',
    smartphone: '<rect x="7" y="2" width="10" height="20" rx="2"/><path d="M11 18h2"/>',
    play: '<polygon points="5 3 19 12 5 21 5 3"/>',
    heart: '<path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.6l-1-1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21l7.8-7.6 1-1a5.5 5.5 0 0 0 0-7.8Z"/>',
    star: '<polygon points="12 2 15 8.5 22 9.3 16.8 14 18.2 21 12 17.4 5.8 21 7.2 14 2 9.3 9 8.5 12 2"/>',
    'file-text': '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M14 2v6h6"/><path d="M8 13h8M8 17h8M8 9h2"/>',
    mic: '<path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2M12 19v3"/>',
    upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m17 8-5-5-5 5M12 3v12"/>'
  };

  const renderIcons = () => {
    document.querySelectorAll('i[data-lucide]').forEach((el) => {
      const name = el.getAttribute('data-lucide');
      const paths = iconPaths[name] || iconPaths.sparkles;
      el.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths}</svg>`;
    });
  };

  const nav = document.querySelector('.site-nav');
  const menu = document.querySelector('[data-menu]');
  if (menu && nav) menu.addEventListener('click', () => nav.classList.toggle('open'));

  const queryApiBase = new URLSearchParams(location.search).get('apiBase');
  const defaultApiBase = location.protocol.startsWith('http') ? location.origin : 'http://127.0.0.1:8088';
  const apiBase = (queryApiBase || window.DAXI_API_BASE || localStorage.getItem('daxi-api-base') || defaultApiBase).replace(/\/$/, '');

  const isEmail = (value) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  const splitContact = (value) => {
    const contact = (value || '').trim();
    return isEmail(contact) ? { email: contact, phone: '' } : { email: '', phone: contact };
  };

  const activeAccountType = (root) => root.querySelector('[data-account-tab].active')?.dataset.accountTab || 'dev';

  const buildLeadPayload = (form, accountType) => {
    const data = new FormData(form);
    if (accountType === 'biz') {
      const contact = splitContact(data.get('biz_contact'));
      return {
        source: 'get-started-business',
        scenario: data.get('scenario') || 'Model API Access',
        company: data.get('company') || 'Business Lead',
        contact_name: data.get('contact_name') || '',
        email: contact.email,
        phone: contact.phone,
        usage_profile: 'Business / partner onboarding',
        notes: `Account type: business. Scenario: ${data.get('scenario') || 'Model API Access'}`,
      };
    }
    const contact = splitContact(data.get('dev_contact'));
    return {
      source: 'get-started-developer',
      scenario: 'Developer API Trial',
      company: 'Individual Developer',
      contact_name: '',
      email: contact.email,
      phone: contact.phone,
      usage_profile: 'Developer API validation',
      notes: `Account type: developer. Access note: ${data.get('access_note') || 'Not provided'}`,
    };
  };

  const submitLead = async (payload) => {
    const response = await fetch(`${apiBase}/public/leads`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    const text = await response.text();
    const result = text ? JSON.parse(text) : {};
    if (!response.ok) {
      throw new Error(result.error || `Request failed: ${response.status}`);
    }
    return result.lead || result;
  };

  const applyLang = (lang) => {
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
    localStorage.setItem('daxi-lang', lang);
    document.querySelectorAll('[data-en]').forEach((el) => {
      const value = el.getAttribute(`data-${lang}`);
      if (value == null) return;
      if (el.hasAttribute('data-html')) el.innerHTML = value;
      else el.textContent = value;
    });
    document.querySelectorAll('[data-lang-current]').forEach((el) => {
      el.textContent = lang === 'zh' ? '中文' : 'English';
    });
    renderIcons();
  };

  const queryLang = new URLSearchParams(location.search).get('lang');
  const initial = queryLang === 'zh' || queryLang === 'en' ? queryLang : (localStorage.getItem('daxi-lang') || 'en');
  applyLang(initial);
  document.querySelectorAll('[data-lang-toggle]').forEach((btn) => {
    btn.addEventListener('click', () => applyLang((localStorage.getItem('daxi-lang') || 'en') === 'en' ? 'zh' : 'en'));
  });

  const register = document.querySelector('[data-register-card]');
  if (register) {
    const tabs = register.querySelectorAll('[data-account-tab]');
    const panels = register.querySelectorAll('[data-account-panel]');
    const scenario = new URLSearchParams(location.search).get('scenario');
    const setTab = (type) => {
      tabs.forEach((tab) => tab.classList.toggle('active', tab.dataset.accountTab === type));
      panels.forEach((panel) => panel.classList.toggle('hidden', panel.dataset.accountPanel !== type));
    };
    tabs.forEach((tab) => tab.addEventListener('click', () => setTab(tab.dataset.accountTab)));
    if (scenario) {
      setTab('biz');
      const select = register.querySelector('select[name="scenario"]');
      if (select) {
        const match = Array.from(select.options).find((opt) => opt.value === scenario || opt.textContent.includes(scenario));
        if (match) select.value = match.value;
      }
    }
    register.querySelector('form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const status = register.querySelector('[data-form-status]');
      const submit = form.querySelector('button[type="submit"]');
      if (!form.reportValidity()) return;
      const active = activeAccountType(register);
      const payload = buildLeadPayload(form, active);
      if (status) {
        status.classList.remove('error');
        status.textContent = document.documentElement.lang === 'zh-CN' ? '正在提交接入申请...' : 'Submitting access request...';
      }
      if (submit) submit.disabled = true;
      let lead;
      try {
        lead = await submitLead(payload);
      } catch (error) {
        if (status) {
          status.classList.add('error');
          status.textContent = document.documentElement.lang === 'zh-CN'
            ? `提交失败：${error.message}`
            : `Submission failed: ${error.message}`;
        }
        if (submit) submit.disabled = false;
        return;
      }
      register.classList.add('submitted');
      register.querySelectorAll('[data-success-type]').forEach((el) => el.classList.toggle('hidden', el.dataset.successType !== active));
      register.querySelectorAll('[data-success-message]').forEach((el) => {
        const lang = document.documentElement.lang === 'zh-CN' ? 'zh' : 'en';
        const prefix = el.getAttribute(`data-${lang}`) || '';
        el.textContent = `${prefix}${lead.public_id || lead.id || '-'}`;
      });
      if (submit) submit.disabled = false;
    });
    register.querySelector('[data-reset-form]')?.addEventListener('click', () => {
      register.classList.remove('submitted');
      const form = register.querySelector('form');
      const status = register.querySelector('[data-form-status]');
      if (status) {
        status.classList.remove('error');
        status.textContent = '';
      }
      form?.reset();
    });
  }

  renderIcons();
})();
