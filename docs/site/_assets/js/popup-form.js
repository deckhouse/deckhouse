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

      this.preferredContact = document.querySelector('input[name="preferred_contact"]');
      this.telegramInput = document.querySelector('input[name="telegram_id"]');
      this.telegramCheckbox = document.querySelector('input[value="telegram"]');
      this.updateContactValue();
      this.initializeCheckbox();
      this.toggleTelegramInput();
      this.telegramCheckbox.addEventListener('change', this.toggleTelegramInput.bind(this));
    }

    initializeCheckbox() {
      const checkboxes = document.querySelectorAll('input[type="checkbox"]');
      checkboxes.forEach(checkbox => {
        checkbox.addEventListener('change', this.updateContactValue.bind(this));
      });
    }

    updateContactValue() {
      let selectedContacts = [];
      const checkboxes = document.querySelectorAll('input[type="checkbox"]');
      checkboxes.forEach(checkbox => {
        if(checkbox.checked) {
          selectedContacts.push(checkbox.value);
        }
      });
      this.preferredContact.value = selectedContacts.join(',');
    }

    toggleTelegramInput() {
      if(this.telegramCheckbox.checked) {
        this.telegramInput.style.display = 'block';
      } else {
        this.telegramInput.style.display = 'none';
        this.telegramInput.value = '';
      }
    }

    initializeOpenModalButton() {
      const openButtons = document.querySelectorAll(`[data-open-modal="${this.modalAttr}"]`);
      openButtons.forEach(button => {
        button.addEventListener('click', this.openModal.bind(this));
      })
    }

    // submitForm(e) {
    //   e.preventDefault();

    //   const FormData = this.serializeData();

    //   const bitrixFields = {
    //     fields: {
    //       'TITLE': 'с сайта документации Deckhouse',
    //       'NAME': FormData.name,
    //       'EMAIL': FormData.email,
    //       'PHONE': FormData.phone,
    //       'COMPANY': FormData.company,
    //       'POST': FormData.position,
    //       'COMMENTS': 'Предпочтительный вид связи: ' + FormData.preferred_contact,
    //     }
    //   }
      
    //   if(this.telegramCheckbox.checked && this.telegramInput.value) {
    //     bitrixFields.fields.COMMENTS += '. Telegram ID: ' + this.telegramInput.value;
    //   }

    //   // const url = 'https://crm.flant.ru/rest/132/bm7uy367wn001kef/crm.lead.add.json';

    // const url = 'https://b24-f0ud24.bitrix24.ru/rest/crm.lead.add.json?auth=8f44b767007618480076182800000001000007c8f5118e27264ab009585d1e51c2c61b';

    //   fetch(url, {
    //     method: 'POST',
    //     headers: {
    //       'Content-Type': 'application/json;charset=utf-8',
    //       Accept: "application/json",
    //     },
    //     body: JSON.stringify(bitrixFields)
    //   })
    //   .then(res => {
    //     if(res.ok) {
    //       res.json();
    //       console.log('успех');
    //     } else {
    //       throw new Error(`${res.status}`);
    //     }
    //   })
    //   .then(data => {
    //     if (data.result) {
    //       this.downloadFile();
    //       this.successSubmit();
    //     } else {
    //       this.errorSubmit();
    //     }
    //   })
    //   .catch(error => {
    //     this.errorSubmit();
    //   })
    // }



    submitForm(e) {
      e.preventDefault();   

      const FormData = this.serializeData();

      // Default Source - Site
      const source_id = 'UC_GAZF8L';
      // const source_id = 'Polina';

      // Default Assigned by - Anna Saprykina
      const assigned_by_id = 7;

      const form_type_labels = {
        'book-your-sessions': 'сессии',
        'callback'          : 'звонка',
        'cs-edition'        : 'CS Edition',
        'demo'              : 'демо',
        'ee-trial'          : 'пробной версии',
        'get-advice'        : 'консультации',
        'partner'           : 'партнёрства',
        'pci-ssc'           : 'отчета PCI SSC',
        'pilot'             : 'пилота',
      }

      const query = [];
      const parts = new URLSearchParams(FormData.current_url);

      if(parts.has('query')) {
        const params = new URLSearchParams(parts.get('query'));

        for(const [key, value] of params.entries()) {
          query[key] = value;
        }
      }

      const bitrixFields = {
        fields: {
          'ASSIGNED_BY_ID': assigned_by_id,
          'SOURCE_ID': source_id,
          'TITLE': '',
        }
      }

      if(FormData.company) {
        bitrixFields.fields['TITLE'] += FormData.company + ' - запрос ';
      }

      if (FormData.form_id === 'partner') {
        const typePartner = {
          'commercial': ' коммерческого',
          'cloud': ' облачного',
          'tech': ' технологического',  
        };
        bitrixFields.fields['TITLE'] += typePartner.FormData['partner_type'] || '';
      }

      // if (form_type_labels.FormData.form_id) {
      //   bitrixFields.fields['TITLE'] += form_type_labels.FormData.form_id;
      // }
  
      bitrixFields.fields['TITLE'] += ' с сайта Deckhouse ';
  
      if (FormData.name) {
        bitrixFields.fields['NAME'] = FormData.name;
      }
  
      if (FormData.email) {
        bitrixFields.fields['EMAIL'] = FormData.email;
      }
  
      if (FormData.phone) {
        bitrixFields.fields['PHONE'] = FormData.phone;
      }
  
      if (FormData.position) {
        bitrixFields.fields['POST'] = FormData.position;
      }
  
      if (FormData.preferred_contact) {
        bitrixFields.fields['COMMENTS'] = `Предпочтительный вид связи: ${FormData.preferred_contact}`;
        if (this.telegramCheckbox.checked && this.telegramInput.value) {
          bitrixFields.fields['COMMENTS'] += `. Telegram ID: ${this.telegramInput.value}`;
        }
      }

      if (FormData.current_url) {
        const params = FormData.current_url.indexOf('?');
        bitrixFields.fields['SOURCE_DESCRIPTION'] = params !== -1 ? FormData.current_url.substring(0, params) : FormData.current_url;
      }

      if (FormData.utm_campaign) {
        bitrixFields.fields['UTM_CAMPAIGN'] = query.utm_campaign;
      }

      if (FormData.utm_medium) {
        bitrixFields.fields['UTM_MEDIUM'] = query.utm_medium;
      }

      if (FormData.utm_source) {
        bitrixFields.fields['UTM_SOURCE'] = query.utm_source;
      }

      if (FormData.utm_term) {
        bitrixFields.fields['UTM_TERM'] = query.utm_term;
      }

      const url = 'https://crm.flant.ru/rest/132/bm7uy367wn001kef/crm.lead.add.json';

      // const url = 'https://b24-f0ud24.bitrix24.ru/rest/crm.lead.add.json?auth=aab4bd67007618480076182800000001000007e6adfb2eee459ffce161d05f6bc9f6da';

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
          res.json();
        } else {
          throw new Error(`${res.status}`);
        }
      })
      .then(data => {
        if (data.result) {
          this.downloadFile();
          this.successSubmit();
        } else {
          this.errorSubmit();
        }
      })  
      .catch(error => {
        this.errorSubmit();
      })
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
