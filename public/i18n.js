// Make sure textarea is hight enough to fit all content.
//
// TODO: on tab make sure the entire entry is visible, sometimes the original on
// the left is larger.
$('.i18n-message textarea').each(function(_, t) {
	$(t).css('height', (t.scrollHeight + 2) + 'px');
})

// Set which translations are visible.
$('#i18n-controls').on('change', function(e) {
	var s = $('#i18n-controls input[name=i18n-show]').filter((_, e) => e.checked)
	switch (s[0].value) {
		case 'all':
			$('.i18n-message').css('display', 'block')
			break;
		case 'untrans':
			$('.i18n-message').css('display', 'none')
			$('.i18n-message[data-status=untrans]').css('display', 'block')
			break;
	}
})

// Show information when textarea is active.
$('.i18n-message textarea').on('focus', function(e) {
	$(this).closest('.i18n-message').find('.i18n-info').css('display', 'block')
})

// Save on blur.
// TODO: Also after user stopped typing for a second
$('.i18n-message textarea').on('blur', function(e) {
	var msg = $(this).closest('.i18n-message')
	msg.find('.i18n-info').css('display', 'none')

	var data = {csrf: CSRF, 'entry.id': msg.attr('data-id')}
	msg.find('textarea').each((_, t) => {
		t = $(t)
		data['entry.' + t.attr('data-field')] = t.val()
	})

	jQuery.ajax({
		method:      'POST',
		url:         location.path,
		dataType:    'json',
		data:        data,

		// TODO: show error if any, set to checkbox onsuccess.
		success: function(data) {
			msg.attr('data-status', 'ok')
		},
		error: function(xhr, settings, e) {
			msg.attr('data-status', 'err')
			msg.find('.i18n-err').attr('title', 'Error: ' + xhr.responseJSON.error)
		},
	})
})
