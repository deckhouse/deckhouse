document.addEventListener("DOMContentLoaded", function() {
  class PopupForm {
    constructor(wrapper) {
      this.wrapper = wrapper;
      this.modalAttr = this.wrapper.dataset.modalWindow;
      this.form = this.wrapper.querySelector('[data-form]');
      this.url = this.form.getAttribute('action');
      this.intro = this.wrapper.querySelector('[data-header-form]');
      this.trick = this.wrapper.querySelector('[data-h0n3y]');
      this.closeBtn = this.wrapper.querySelector('[data-close-btn]')
      this.closeBg = this.wrapper.querySelector('[data-close-bg]');
      this.success = this.wrapper.querySelector('[data-success-message]');
      this.error = this.wrapper.querySelector('[data-error-message]');
      this.initializeOpenModalButton();
      this.form.addEventListener('submit', this.submitForm.bind(this));
      this.closeBtn.addEventListener('click', this.closeModal.bind(this));
      this.closeBg.addEventListener('click', this.closeModal.bind(this));
    }

    initializeOpenModalButton() {
      const openButtons = document.querySelectorAll(`[data-open-modal="${this.modalAttr}"]`);
      openButtons.forEach(button => {
        button.addEventListener('click', this.openModal.bind(this));
      })
    }

    submitForm(e) {
      e.preventDefault();

      if (this.trick.value !== '') {
        this.errorSubmit();
      } else {
        PostData(this.url, this.serializeData()).then(res => {
          if (res.ok) {
            this.successSubmit();
          } else {
            this.errorSubmit();
          }
        });
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
  }

  const wrapper = document.querySelectorAll('[data-modal-window]');

  wrapper.forEach(item => {
    new PopupForm(item);
  })

  async function PostData(url, data) {
    const res = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json;charset=utf-8',
        Accept: "application/json",
      },
      body: JSON.stringify(data)
    })
    return res
  }
})
