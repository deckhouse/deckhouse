document.addEventListener('DOMContentLoaded', function () {
    const activeStep = document.querySelector('.gs-steps__point-num_active');
    if (!activeStep) return;

    function centerActiveStep(behavior) {
        activeStep.scrollIntoView({
            inline: 'center',
            block: 'nearest',
            behavior: behavior
        });
    }

    centerActiveStep('smooth');
    window.addEventListener('resize', function () {
        centerActiveStep('auto');
    });
});
