document.addEventListener("DOMContentLoaded", function() {
  class PopupForm {
    constructor(wrapper) {
      this.wrapper = wrapper;
      this.modalAttr = this.wrapper.dataset.modalWindow;
      this.form = this.wrapper.querySelector('[data-form]');
      this.url = this.form.getAttribute('action');
      this.intro = this.wrapper.querySelector('[data-header-form]');
      this.closeBtn = this.wrapper.querySelector('[data-close-btn]')
      this.closeBg = this.wrapper.querySelector('[data-close-bg]');
      this.success = this.wrapper.querySelector('[data-success-message]');
      this.error = this.wrapper.querySelector('[data-error-message]');
      this.initializeOpenModalButton();
      this.form.addEventListener('submit', this.submitForm.bind(this));
      this.closeBtn.addEventListener('click', this.closeModal.bind(this));
      this.closeBg.addEventListener('click', this.closeModal.bind(this));

      this.preferredContact = this.wrapper.querySelector('input[name="preferred_contact"]');
      this.telegramInput = this.wrapper.querySelector('input[name="telegram_id"]');
      this.telegramCheckbox = this.wrapper.querySelector('input[value="telegram"]');
      this.checkboxes = this.wrapper.querySelectorAll('input[type="checkbox"]');
      this.updateContactValue();
      this.initializeCheckbox();
      this.toggleTelegramInput();
      if(this.telegramCheckbox) {
        this.telegramCheckbox.addEventListener('change', this.toggleTelegramInput.bind(this));
      }
    }

    initializeCheckbox() {
      this.checkboxes.forEach(checkbox => {
        checkbox.addEventListener('change', this.updateContactValue.bind(this));
      });
    }

    updateContactValue() {
      if(this.preferredContact) {
        let selectedContacts = [];
        this.checkboxes.forEach(checkbox => {
          if(checkbox.checked) {
            selectedContacts.push(checkbox.value);
          }
        });
        this.preferredContact.value = selectedContacts.join(',');
      }
    }

    toggleTelegramInput() {
      if(this.telegramCheckbox) {
        if(this.telegramCheckbox.checked) {
          this.telegramInput.style.display = 'block';
        } else {
          this.telegramInput.style.display = 'none';
          this.telegramInput.value = '';
        }
      }
    }

    initializeOpenModalButton() {
      const openButtons = document.querySelectorAll(`[data-open-modal="${this.modalAttr}"]`);
      openButtons.forEach(button => {
        button.addEventListener('click', this.openModal.bind(this));
      })
    }

    submitForm(e) {
      e.preventDefault();   

      const FormData = this.serializeData();

      // Default Source - Site
      const source_id = 'UC_GAZF8L';

      // Default Assigned by - Anna Saprykina
      const assigned_by_id = 7;

      const bitrixFields = {
        fields: {
          'ASSIGNED_BY_ID': assigned_by_id,
          'SOURCE_ID': source_id,
          'TITLE': '',
        }
      }

      const modalAttr = this.wrapper.dataset.modalWindow;

      if(modalAttr == 'request_access') {
        if(FormData.company) {
          bitrixFields.fields['TITLE'] += FormData.company + ' - запрос бесплатного триала';
        }
      } else {
        if(FormData.company) {
          bitrixFields.fields['TITLE'] += FormData.company + ' - запрос ';
        }
    
        bitrixFields.fields['TITLE'] += 'с сайта Deckhouse ';
      }
  
      if (FormData.name) {
        bitrixFields.fields['NAME'] = FormData.name;
      }

      if (FormData.email) {
        bitrixFields.fields['EMAIL'] = [
          {
            'VALUE': FormData.email,
            'VALUE_TYPE': 'WORK',
          }
        ]
      }

      if (FormData.phone) {
        bitrixFields.fields['PHONE'] = [
          {
            'VALUE': FormData.phone,
            'VALUE_TYPE': 'WORK',
          }
        ]
      }
  
      if (FormData.position) {
        bitrixFields.fields['POST'] = FormData.position;
      }
  
      if (FormData.preferred_contact) {
        bitrixFields.fields['COMMENTS'] = `Предпочтительный вид связи: ${FormData.preferred_contact}`;
        if (this.telegramCheckbox.checked && this.telegramInput.value) {
          bitrixFields.fields['COMMENTS'] += `. Telegram ID: ${this.telegramInput.value}`;
          bitrixFields.fields['IM'] = [
            {
              'VALUE': this.telegramInput.value,
              'VALUE_TYPE': 'TELEGRAM'
            }
          ]
        }
      }

      if (FormData.referer_url) {
        const params = FormData.referer_url.indexOf('?');
        bitrixFields.fields['SOURCE_DESCRIPTION'] = params !== -1 ? FormData.referer_url.substring(0, params) : FormData.referer_url;
      }

      const query = {};
      const parts = new URL(FormData.referer_url).searchParams;

      parts.forEach((value, key) => {
        if(key.startsWith('utm_')) {
          query[key] = value;
        }
      })

      if (query.utm_campaign) {
        bitrixFields.fields['UTM_CAMPAIGN'] = query.utm_campaign;
      }

      if (query.utm_medium) {
        bitrixFields.fields['UTM_MEDIUM'] = query.utm_medium;
      }

      if (query.utm_source) {
        bitrixFields.fields['UTM_SOURCE'] = query.utm_source;
      }

      if (query.utm_term) {
        bitrixFields.fields['UTM_TERM'] = query.utm_term;
      }

      function createCookie(name, value, days) {
        const date = new Date(Date.now() + (days * 24 * 60 * 60 * 1000));
        const expires = 'expires=' + date.toUTCString();
        document.cookie = name + '=' + encodeURIComponent(value) + ';' + expires + ';path=/';
      }

      const utmAccepted = ['utm_campaign', 'utm_medium', 'utm_source', 'utm_term'];
      utmAccepted.forEach(param => {
        if(query[param]) {
          if(!document.cookie.includes(param + '=')) {
            createCookie(param, query[param], 28);
          }
        } else {
          createCookie(param, '', -1);
        }
      })

      function themeFormValidation(data) {
        const spamPattern = /(\b(SELECT|INSERT|UPDATE|DELETE|DROP|CASE|WHEN|SLEEP|--|\|\||OR|AND|CHR\()\b)/i;

        function checkingValue(value) {
          if(!value) return false;

          if(typeof value === 'string') {
            return spamPattern.test(value);
          }

          if(typeof value === 'number') {
            return spamPattern.test(value.toString());
          }

          if(Array.isArray(value)) {
            return value.some(checkingValue);
          }

          if(typeof value === 'object' && value !== null) {
            return Object.values(value).some(checkingValue);
          }

          return false;
        }

        return checkingValue(data);
      }

      let isSpam = false;

      for(const fieldsValue in bitrixFields.fields) {
        if(themeFormValidation(bitrixFields.fields[fieldsValue])) {
          isSpam = true;
          break;
        }
      }

      if(!isSpam) {
        const url = 'https://crm.flant.ru/rest/132/bm7uy367wn001kef/crm.lead.add.json';

        fetch(url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json;charset=utf-8',
            Accept: "application/json",
          },
          body: JSON.stringify(bitrixFields)
        })
        .then(res => {
          if(res.ok) {
            this.downloadFile();
            this.successSubmit();
          } else {
            this.errorSubmit();
          }
        })
      } else {
        this.errorSubmit();
      }
    }

    serializeData() {
      let data = new FormData(this.form);
      let serializedData = Object.fromEntries(data.entries());
      serializedData.referer_url = window.location.href;
      return serializedData;
    }

    successSubmit() {
      this.intro.style.display = 'none';
      this.success.style.display = 'block';
    }

    errorSubmit() {
      this.intro.style.display = 'none';
      this.error.style.display = 'block';
    }

    openModal(e) {
      e.preventDefault();
      this.wrapper.style.display = 'flex';
      document.addEventListener('keydown', this.closeModalOnEscape.bind(this));
    }

    closeModal(e) {
      e.preventDefault();
      this.wrapper.style.display = 'none';
      this.intro.style.display = 'block';
      this.success.style.display = 'none';
      this.error.style.display = 'none';
    }

    closeModalOnEscape(e) {
      if (e.key === 'Escape') {
        this.wrapper.style.display = 'none';
        this.intro.style.display = 'block';
        this.success.style.display = 'none';
        this.error.style.display = 'none';
      }
    }

    downloadFile() {
      if (!this.form.hasAttribute('data-download-file')) return

      const fileName = this.form.getAttribute('data-download-file');
      const downloadFileButton = this.success.querySelector('button.button');
      const a = document.createElement('a')

      a.href = `/reports/pci_ssc_reports_files/${fileName}`;
      a.download = fileName;
      a.click();

      downloadFileButton.addEventListener('click', () => {
        a.click();
      })
    }
  }

  const wrapper = document.querySelectorAll('[data-modal-window]');

  wrapper.forEach(item => {
    new PopupForm(item);
  })
})
