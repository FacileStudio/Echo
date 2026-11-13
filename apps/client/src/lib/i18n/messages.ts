export type Message = string | { one: string; other: string };
export type Locale = 'fr' | 'en' | 'es' | 'de';
export type Catalogs<C> = Record<Locale, C>;

type Row = readonly [en: Message, fr: Message, es: Message, de: Message];

const rows = {
	'login.signIn': ['Sign in', 'Connexion', 'Iniciar sesión', 'Anmelden'],
	'login.description': ['Echo uses the studio single sign-on.', 'Echo utilise la connexion unique du studio.', 'Echo usa el inicio de sesión único del estudio.', 'Echo nutzt die zentrale Anmeldung des Studios.'],
	'login.continue': ['Continue with Facile', 'Continuer avec Facile', 'Continuar con Facile', 'Weiter mit Facile'],
	'title.login': ['Sign in · Echo', 'Connexion · Echo', 'Iniciar sesión · Echo', 'Anmelden · Echo'],
	'title.room': ['{slug} · Echo', '{slug} · Echo', '{slug} · Echo', '{slug} · Echo'],
	'title.call': ['Call · Echo', 'Appel · Echo', 'Llamada · Echo', 'Anruf · Echo'],
	'title.history': ['History · {slug} · Echo', 'Historique · {slug} · Echo', 'Historial · {slug} · Echo', 'Verlauf · {slug} · Echo'],

	'home.tagline': ['Video rooms for the studio.', 'Des salles vidéo pour le studio.', 'Salas de vídeo para el estudio.', 'Videoräume für das Studio.'],
	'home.roomName': ['Room name', 'Nom de la salle', 'Nombre de la sala', 'Name des Raums'],
	'home.open': ['Open room', 'Ouvrir la salle', 'Abrir la sala', 'Raum öffnen'],
	'home.roomRequired': [
		'Enter a room name or paste a room link',
		'Saisissez un nom de salle ou collez un lien de salle',
		'Escribe un nombre de sala o pega un enlace de sala',
		'Gib einen Raumnamen ein oder füge einen Raumlink ein'
	],
	'auth.failed': ['Login failed', 'Échec de la connexion', 'Error al iniciar sesión', 'Anmeldung fehlgeschlagen'],

	'room.join': ['Join the room.', 'Rejoignez la salle.', 'Únete a la sala.', 'Tritt dem Raum bei.'],
	'room.tryAgain': ['Try again', 'Réessayer', 'Reintentar', 'Erneut versuchen'],
	'room.couldNotJoin': ['Could not join', 'Impossible de rejoindre', 'No se pudo entrar', 'Beitritt nicht möglich'],
	'room.back': ['Back', 'Retour', 'Volver', 'Zurück'],
	'room.history': ['History', 'Historique', 'Historial', 'Verlauf'],
	'room.joiningAs': ['Joining as {name}…', 'Connexion en tant que {name}…', 'Entrando como {name}…', 'Beitritt als {name}…'],
	'room.yourName': ['Your name', 'Votre nom', 'Tu nombre', 'Dein Name'],
	'room.joinAsGuest': ['Join as guest', 'Rejoindre en invité', 'Entrar como invitado', 'Als Gast beitreten'],
	'room.backToEcho': ['Back to Echo', 'Retour à Echo', 'Volver a Echo', 'Zurück zu Echo'],
	'room.tooMany': [
		'Too many join attempts. Wait a minute and try again.',
		'Trop de tentatives. Patientez une minute puis réessayez.',
		'Demasiados intentos. Espera un minuto y vuelve a intentarlo.',
		'Zu viele Beitrittsversuche. Warte eine Minute und versuche es erneut.'
	],
	'room.tokenFailed': [
		'The join request failed ({status})',
		'La demande pour rejoindre a échoué ({status})',
		'Falló la solicitud para entrar ({status})',
		'Die Beitrittsanfrage ist fehlgeschlagen ({status})'
	],

	'embed.left': ['You left the call.', 'Vous avez quitté l’appel.', 'Has salido de la llamada.', 'Du hast den Anruf verlassen.'],
	'embed.cannotJoin': ['Cannot join', 'Impossible de rejoindre', 'No se puede entrar', 'Beitritt nicht möglich'],
	'embed.noKey': [
		'This call link has no encryption key. Echo calls are end-to-end encrypted, so the app that opened this call must pass the key. Open the call again from your app.',
		'Ce lien d’appel n’a pas de clé de chiffrement. Les appels Echo sont chiffrés de bout en bout, donc l’application qui a ouvert cet appel doit transmettre la clé. Rouvrez l’appel depuis votre application.',
		'Este enlace de llamada no tiene clave de cifrado. Las llamadas de Echo están cifradas de extremo a extremo, así que la aplicación que abrió esta llamada debe pasar la clave. Vuelve a abrir la llamada desde tu aplicación.',
		'Dieser Anruflink enthält keinen Verschlüsselungsschlüssel. Echo-Anrufe sind Ende-zu-Ende-verschlüsselt, deshalb muss die App, die den Anruf geöffnet hat, den Schlüssel mitgeben. Öffne den Anruf erneut in deiner App.'
	],
	'embed.noToken': [
		'This call link has no join token. Open the call again from your app.',
		'Ce lien d’appel n’a pas de jeton d’accès. Rouvrez l’appel depuis votre application.',
		'Este enlace de llamada no tiene token de acceso. Vuelve a abrir la llamada desde tu aplicación.',
		'Dieser Anruflink enthält kein Zugangstoken. Öffne den Anruf erneut in deiner App.'
	],

	'live.browserTitle': ['This browser cannot join', 'Ce navigateur ne peut pas rejoindre', 'Este navegador no puede entrar', 'Dieser Browser kann nicht beitreten'],
	'live.browserBody': [
		'Echo calls are end-to-end encrypted, and this browser cannot encrypt media. Open the link in a recent Chrome, Edge, Firefox or Safari.',
		'Les appels Echo sont chiffrés de bout en bout et ce navigateur ne sait pas chiffrer les médias. Ouvrez le lien dans une version récente de Chrome, Edge, Firefox ou Safari.',
		'Las llamadas de Echo están cifradas de extremo a extremo y este navegador no puede cifrar el contenido multimedia. Abre el enlace en una versión reciente de Chrome, Edge, Firefox o Safari.',
		'Echo-Anrufe sind Ende-zu-Ende-verschlüsselt, und dieser Browser kann Medien nicht verschlüsseln. Öffne den Link in einer aktuellen Version von Chrome, Edge, Firefox oder Safari.'
	],
	'live.noMedia': [
		'Camera or microphone unavailable. You joined without publishing.',
		'Caméra ou micro indisponible. Vous avez rejoint sans publier.',
		'Cámara o micrófono no disponible. Has entrado sin publicar.',
		'Kamera oder Mikrofon nicht verfügbar. Du bist ohne Übertragung beigetreten.'
	],
	'live.cannotDecrypt': ['Cannot decrypt', 'Déchiffrement impossible', 'No se puede descifrar', 'Entschlüsselung nicht möglich'],
	'live.mediaProblem': ['Media problem', 'Problème de média', 'Problema de medios', 'Medienproblem'],
	'live.screenFailed': [
		'Screen sharing failed: {reason}',
		'Le partage d’écran a échoué : {reason}',
		'Falló el uso compartido de pantalla: {reason}',
		'Die Bildschirmfreigabe ist fehlgeschlagen: {reason}'
	],
	'crypt.own': [
		'Can’t decrypt this call: your link is missing the right key. Ask for the full link, including everything after #.',
		'Impossible de déchiffrer cet appel : votre lien n’a pas la bonne clé. Demandez le lien complet, avec tout ce qui suit le #.',
		'No se puede descifrar esta llamada: a tu enlace le falta la clave correcta. Pide el enlace completo, con todo lo que va después de #.',
		'Dieser Anruf kann nicht entschlüsselt werden: In deinem Link fehlt der richtige Schlüssel. Frage nach dem vollständigen Link, einschließlich allem nach dem #.'
	],
	'crypt.others': [
		'Can’t decrypt {names}: they joined with a different key. Send them the full link.',
		'Impossible de déchiffrer {names} : ils ont rejoint avec une autre clé. Envoyez-leur le lien complet.',
		'No se puede descifrar a {names}: entraron con otra clave. Envíales el enlace completo.',
		'{names} kann nicht entschlüsselt werden: Sie sind mit einem anderen Schlüssel beigetreten. Schicke ihnen den vollständigen Link.'
	],

	'call.encryptedTip': ['End-to-end encrypted', 'Chiffré de bout en bout', 'Cifrado de extremo a extremo', 'Ende-zu-Ende-verschlüsselt'],
	'call.encrypted': ['Encrypted', 'Chiffré', 'Cifrado', 'Verschlüsselt'],
	'call.mute': ['Mute microphone', 'Couper le micro', 'Silenciar el micrófono', 'Mikrofon stummschalten'],
	'call.unmute': ['Unmute microphone', 'Réactiver le micro', 'Activar el micrófono', 'Mikrofon einschalten'],
	'call.camOff': ['Turn camera off', 'Couper la caméra', 'Apagar la cámara', 'Kamera ausschalten'],
	'call.camOn': ['Turn camera on', 'Activer la caméra', 'Encender la cámara', 'Kamera einschalten'],
	'call.stopShare': ['Stop sharing screen', 'Arrêter le partage d’écran', 'Dejar de compartir pantalla', 'Bildschirmfreigabe beenden'],
	'call.share': ['Share screen', 'Partager l’écran', 'Compartir pantalla', 'Bildschirm freigeben'],
	'call.hideChat': ['Hide chat', 'Masquer le chat', 'Ocultar el chat', 'Chat ausblenden'],
	'call.showChat': ['Show chat', 'Afficher le chat', 'Mostrar el chat', 'Chat anzeigen'],
	'call.leave': ['Leave', 'Quitter', 'Salir', 'Verlassen'],

	'header.people': [
		{ one: '{count} person in the call', other: '{count} people in the call' },
		{ one: '{count} personne dans l’appel', other: '{count} personnes dans l’appel' },
		{ one: '{count} persona en la llamada', other: '{count} personas en la llamada' },
		{ one: '{count} Person im Anruf', other: '{count} Personen im Anruf' }
	],
	'header.copyLink': ['Copy link', 'Copier le lien', 'Copiar enlace', 'Link kopieren'],
	'header.copied': ['Copied', 'Copié', 'Copiado', 'Kopiert'],

	'countdown.endsIn': ['Call ends in {time}', 'L’appel se termine dans {time}', 'La llamada termina en {time}', 'Der Anruf endet in {time}'],
	'countdown.soonTitle': ['This call ends soon', 'Cet appel se termine bientôt', 'Esta llamada termina pronto', 'Dieser Anruf endet bald'],
	'countdown.soonBody': [
		'Each meeting has a time limit on the host’s plan. The call closes for everyone in {time}, and a new one can start in this room 2 minutes later.',
		'Chaque réunion a une durée limitée avec l’offre de l’hôte. L’appel se ferme pour tout le monde dans {time}, et un nouvel appel peut démarrer dans cette salle 2 minutes plus tard.',
		'Cada reunión tiene un límite de tiempo en el plan del anfitrión. La llamada se cierra para todos en {time}, y una nueva puede empezar en esta sala 2 minutos después.',
		'Jedes Meeting hat im Tarif des Gastgebers ein Zeitlimit. Der Anruf wird in {time} für alle beendet, und 2 Minuten später kann in diesem Raum ein neuer beginnen.'
	],

	'chat.empty': [
		'No messages yet. Chat is visible only in this call.',
		'Aucun message pour l’instant. Le chat n’est visible que dans cet appel.',
		'Aún no hay mensajes. El chat solo es visible en esta llamada.',
		'Noch keine Nachrichten. Der Chat ist nur in diesem Anruf sichtbar.'
	],
	'chat.placeholder': ['Message', 'Message', 'Mensaje', 'Nachricht'],
	'chat.label': ['Chat message', 'Message du chat', 'Mensaje del chat', 'Chatnachricht'],
	'chat.send': ['Send', 'Envoyer', 'Enviar', 'Senden'],
	'chat.unknown': ['Unknown', 'Inconnu', 'Desconocido', 'Unbekannt'],

	'tile.yourScreen': ['Your screen', 'Votre écran', 'Tu pantalla', 'Dein Bildschirm'],
	'tile.screenOf': ['{name}’s screen', 'Écran de {name}', 'Pantalla de {name}', 'Bildschirm von {name}'],
	'tile.you': ['{name} (you)', '{name} (vous)', '{name} (tú)', '{name} (du)'],
	'tile.muted': ['Muted', 'Micro coupé', 'Silenciado', 'Stummgeschaltet'],
	'stage.connecting': ['Connecting…', 'Connexion…', 'Conectando…', 'Verbindung wird hergestellt…'],
	'audio.enable': ['Enable sound', 'Activer le son', 'Activar el sonido', 'Ton aktivieren'],

	'plan.title': ['Plan limit reached', 'Limite de l’offre atteinte', 'Límite del plan alcanzado', 'Tariflimit erreicht'],
	'plan.upgrade': ['Upgrade the plan', 'Changer d’offre', 'Mejorar el plan', 'Tarif upgraden'],
	'plan.full': [
		'This call is full. The host’s plan allows {count} people per call.',
		'Cet appel est complet. L’offre de l’hôte autorise {count} personnes par appel.',
		'Esta llamada está llena. El plan del anfitrión permite {count} personas por llamada.',
		'Dieser Anruf ist voll. Der Tarif des Gastgebers erlaubt {count} Personen pro Anruf.'
	],
	'plan.noRecording': [
		'Recording is not included in the host’s plan.',
		'L’enregistrement n’est pas inclus dans l’offre de l’hôte.',
		'La grabación no está incluida en el plan del anfitrión.',
		'Die Aufzeichnung ist im Tarif des Gastgebers nicht enthalten.'
	],
	'plan.minutes': [
		'The last call in this room reached its {count}-minute limit. On the host’s plan each meeting lasts up to {count} minutes, and a new one can start 2 minutes after the last one ends.',
		'Le dernier appel de cette salle a atteint sa limite de {count} minutes. Avec l’offre de l’hôte, chaque réunion dure jusqu’à {count} minutes, et une nouvelle peut démarrer 2 minutes après la fin de la précédente.',
		'La última llamada de esta sala alcanzó su límite de {count} minutos. En el plan del anfitrión cada reunión dura hasta {count} minutos, y una nueva puede empezar 2 minutos después de que termine la anterior.',
		'Der letzte Anruf in diesem Raum hat sein Limit von {count} Minuten erreicht. Im Tarif des Gastgebers dauert jedes Meeting bis zu {count} Minuten, und 2 Minuten nach dem Ende kann ein neues beginnen.'
	],
	'plan.minutesGeneric': [
		'The host’s plan limits how long a call can last.',
		'L’offre de l’hôte limite la durée d’un appel.',
		'El plan del anfitrión limita la duración de una llamada.',
		'Der Tarif des Gastgebers begrenzt die Dauer eines Anrufs.'
	],
	'plan.default': [
		'Your plan does not allow this.',
		'Votre offre ne permet pas cela.',
		'Tu plan no permite esto.',
		'Dein Tarif erlaubt das nicht.'
	],

	'participants.none': [
		'No participants recorded for this call.',
		'Aucun participant enregistré pour cet appel.',
		'No hay participantes registrados en esta llamada.',
		'Für diesen Anruf sind keine Teilnehmenden erfasst.'
	],
	'participants.participant': ['Participant', 'Participant', 'Participante', 'Teilnehmende'],
	'participants.joined': ['Joined', 'Arrivé', 'Entró', 'Beigetreten'],
	'participants.left': ['Left', 'Parti', 'Salió', 'Verlassen'],
	'participants.still': ['Still connected', 'Toujours connecté', 'Sigue conectado', 'Noch verbunden'],
	'transcript.none': [
		'No transcript. Calls are end-to-end encrypted, so each browser transcribes only its own microphone, and only when the organizer turned it on and the person accepted.',
		'Pas de transcription. Les appels sont chiffrés de bout en bout : chaque navigateur ne transcrit que son propre micro, et seulement si l’organisateur l’a activé et que la personne a accepté.',
		'Sin transcripción. Las llamadas están cifradas de extremo a extremo: cada navegador solo transcribe su propio micrófono, y solo si el organizador lo activó y la persona aceptó.',
		'Kein Transkript. Anrufe sind Ende-zu-Ende-verschlüsselt: Jeder Browser transkribiert nur das eigene Mikrofon, und nur wenn der Organisator es eingeschaltet und die Person zugestimmt hat.'
	],

	'history.started': ['Started', 'Début', 'Inicio', 'Beginn'],
	'history.duration': ['Duration', 'Durée', 'Duración', 'Dauer'],
	'history.recording': ['Recording', 'Enregistrement', 'Grabación', 'Aufzeichnung'],
	'history.details': ['Details', 'Détails', 'Detalles', 'Details'],
	'history.inProgress': ['In progress', 'En cours', 'En curso', 'Läuft'],
	'history.available': ['Available', 'Disponible', 'Disponible', 'Verfügbar'],
	'history.none': ['None', 'Aucun', 'Ninguna', 'Keine'],
	'history.open': ['Open', 'Ouvrir', 'Abrir', 'Öffnen'],
	'historyPage.title': ['History of “{slug}”', 'Historique de « {slug} »', 'Historial de «{slug}»', 'Verlauf von „{slug}“'],
	'historyPage.description': [
		'Past calls in this room, newest first.',
		'Appels passés dans cette salle, les plus récents d’abord.',
		'Llamadas pasadas en esta sala, las más recientes primero.',
		'Frühere Anrufe in diesem Raum, die neuesten zuerst.'
	],
	'historyPage.back': ['Back to the room', 'Retour à la salle', 'Volver a la sala', 'Zurück zum Raum'],
	'historyPage.ownerOnly': ['Owner only', 'Réservé au propriétaire', 'Solo el propietario', 'Nur für den Eigentümer'],
	'historyPage.ownerOnlyBody': [
		'Only the room owner can see this history. Sign in with the account that created “{slug}”.',
		'Seul le propriétaire de la salle peut voir cet historique. Connectez-vous avec le compte qui a créé « {slug} ».',
		'Solo el propietario de la sala puede ver este historial. Inicia sesión con la cuenta que creó «{slug}».',
		'Nur der Eigentümer des Raums kann diesen Verlauf sehen. Melde dich mit dem Konto an, das „{slug}“ erstellt hat.'
	],
	'historyPage.loadFailed': ['Could not load history', 'Impossible de charger l’historique', 'No se pudo cargar el historial', 'Verlauf konnte nicht geladen werden'],
	'historyPage.emptyTitle': ['No calls yet', 'Aucun appel pour l’instant', 'Aún no hay llamadas', 'Noch keine Anrufe'],
	'historyPage.emptyBody': [
		'Once a call has taken place in this room, it will show up here.',
		'Dès qu’un appel aura eu lieu dans cette salle, il apparaîtra ici.',
		'Cuando haya habido una llamada en esta sala, aparecerá aquí.',
		'Sobald in diesem Raum ein Anruf stattgefunden hat, erscheint er hier.'
	],
	'historyPage.openRoom': ['Open the room', 'Ouvrir la salle', 'Abrir la sala', 'Raum öffnen'],

	'callPage.title': ['Call details', 'Détails de l’appel', 'Detalles de la llamada', 'Anrufdetails'],
	'callPage.home': ['Home', 'Accueil', 'Inicio', 'Start'],
	'callPage.ownerOnlyBody': [
		'Only the room owner and the people who were in this call can see it. Sign in with your Facile account.',
		'Seuls le propriétaire de la salle et les personnes présentes à cet appel peuvent le voir. Connectez-vous avec votre compte Facile.',
		'Solo el propietario de la sala y las personas que estuvieron en esta llamada pueden verla. Inicia sesión con tu cuenta Facile.',
		'Nur der Eigentümer des Raums und die Personen, die an diesem Anruf teilgenommen haben, können ihn sehen. Melde dich mit deinem Facile-Konto an.'
	],
	'callPage.loadFailed': ['Could not load call', 'Impossible de charger l’appel', 'No se pudo cargar la llamada', 'Anruf konnte nicht geladen werden'],
	'callPage.participants': ['Participants', 'Participants', 'Participantes', 'Teilnehmende'],
	'callPage.recording': ['Recording', 'Enregistrement', 'Grabación', 'Aufzeichnung'],
	'callPage.downloadFailed': ['Download failed', 'Échec du téléchargement', 'Falló la descarga', 'Download fehlgeschlagen'],
	'callPage.preparing': [
		'Preparing the download…',
		'Préparation du téléchargement…',
		'Preparando la descarga…',
		'Download wird vorbereitet…'
	],
	'callPage.download': [
		'Download the video (MP4)',
		'Télécharger la vidéo (MP4)',
		'Descargar el vídeo (MP4)',
		'Video herunterladen (MP4)'
	],
	'callPage.noRecording': [
		'No recording. Calls are end-to-end encrypted, so only the organizer’s browser can record, and only the voices of people who accepted.',
		'Pas d’enregistrement. Les appels sont chiffrés de bout en bout : seul le navigateur de l’organisateur peut enregistrer, et uniquement la voix des personnes qui ont accepté.',
		'Sin grabación. Las llamadas están cifradas de extremo a extremo: solo el navegador del organizador puede grabar, y solo la voz de quienes aceptaron.',
		'Keine Aufzeichnung. Anrufe sind Ende-zu-Ende-verschlüsselt: Nur der Browser des Organisators kann aufzeichnen, und nur die Stimmen der Personen, die zugestimmt haben.'
	],
	'callPage.summary': ['AI summary', 'Résumé IA', 'Resumen con IA', 'KI-Zusammenfassung'],
	'callPage.transcript': ['Transcript', 'Transcription', 'Transcripción', 'Transkript'],

	'summary.unavailable': ['Summary unavailable', 'Résumé indisponible', 'Resumen no disponible', 'Zusammenfassung nicht verfügbar'],
	'summary.notConfigured': [
		'AI summaries are unavailable right now. Try again later.',
		'Les résumés IA sont indisponibles pour le moment. Réessayez plus tard.',
		'Los resúmenes con IA no están disponibles ahora. Inténtalo más tarde.',
		'KI-Zusammenfassungen sind gerade nicht verfügbar. Versuche es später erneut.'
	],
	'summary.noTranscript': [
		'This call has no transcript, so there is nothing to summarise.',
		'Cet appel n’a pas de transcription, il n’y a donc rien à résumer.',
		'Esta llamada no tiene transcripción, así que no hay nada que resumir.',
		'Dieser Anruf hat kein Transkript, daher gibt es nichts zusammenzufassen.'
	],
	'summary.ownerOnly': [
		'Only the room owner can generate a summary.',
		'Seul le propriétaire de la salle peut générer un résumé.',
		'Solo el propietario de la sala puede generar un resumen.',
		'Nur der Eigentümer des Raums kann eine Zusammenfassung erstellen.'
	],
	'summary.generating': [
		'Generating the summary. This can take up to two minutes.',
		'Génération du résumé. Cela peut prendre jusqu’à deux minutes.',
		'Generando el resumen. Puede tardar hasta dos minutos.',
		'Die Zusammenfassung wird erstellt. Das kann bis zu zwei Minuten dauern.'
	],
	'summary.generatedBy': [
		'Generated by {model} on {date}',
		'Généré par {model} le {date}',
		'Generado por {model} el {date}',
		'Erstellt von {model} am {date}'
	],
	'summary.noneEncrypted': [
		'No summary. This call has no transcript to summarise: it needs transcription turned on during the call.',
		'Pas de résumé. Cet appel n’a pas de transcription à résumer : il faut activer la transcription pendant l’appel.',
		'Sin resumen. Esta llamada no tiene transcripción que resumir: hay que activar la transcripción durante la llamada.',
		'Keine Zusammenfassung. Dieser Anruf hat kein Transkript zum Zusammenfassen: Die Transkription muss während des Anrufs eingeschaltet werden.'
	],
	'summary.none': [
		'No summary has been generated for this call yet.',
		'Aucun résumé n’a encore été généré pour cet appel.',
		'Todavía no se ha generado un resumen de esta llamada.',
		'Für diesen Anruf wurde noch keine Zusammenfassung erstellt.'
	],
	'summary.regenerate': ['Regenerate summary', 'Régénérer le résumé', 'Regenerar el resumen', 'Zusammenfassung neu erstellen'],
	'summary.generate': ['Generate summary', 'Générer le résumé', 'Generar el resumen', 'Zusammenfassung erstellen'],

	'title.meetings': ['Meetings · Echo', 'Réunions · Echo', 'Reuniones · Echo', 'Meetings · Echo'],
	'meetings.title': ['Your meetings', 'Vos réunions', 'Tus reuniones', 'Deine Meetings'],
	'meetings.description': [
		'The rooms you own or joined while signed in, newest first.',
		'Les salles que vous possédez ou que vous avez rejointes en étant connecté, les plus récentes d’abord.',
		'Las salas que posees o a las que te uniste con sesión iniciada, las más recientes primero.',
		'Die Räume, die dir gehören oder denen du angemeldet beigetreten bist, die neuesten zuerst.'
	],
	'meetings.signInTitle': ['Sign in to see your meetings', 'Connectez-vous pour voir vos réunions', 'Inicia sesión para ver tus reuniones', 'Melde dich an, um deine Meetings zu sehen'],
	'meetings.signInBody': [
		'Your history, recordings and transcripts follow your Facile account.',
		'Votre historique, vos enregistrements et vos transcriptions suivent votre compte Facile.',
		'Tu historial, tus grabaciones y tus transcripciones siguen a tu cuenta Facile.',
		'Dein Verlauf, deine Aufzeichnungen und deine Transkripte gehören zu deinem Facile-Konto.'
	],
	'meetings.emptyTitle': ['No meetings yet', 'Aucune réunion pour l’instant', 'Aún no hay reuniones', 'Noch keine Meetings'],
	'meetings.emptyBody': [
		'Calls you join while signed in will appear here.',
		'Les appels que vous rejoignez en étant connecté apparaîtront ici.',
		'Las llamadas a las que te unas con sesión iniciada aparecerán aquí.',
		'Anrufe, denen du angemeldet beitrittst, erscheinen hier.'
	],
	'meetings.previous': ['Previous', 'Précédent', 'Anterior', 'Zurück'],
	'meetings.next': ['Next', 'Suivant', 'Siguiente', 'Weiter'],
	'meetings.pageOf': ['Page {page} of {pages}', 'Page {page} sur {pages}', 'Página {page} de {pages}', 'Seite {page} von {pages}'],
	'meetings.link': ['Your meetings', 'Vos réunions', 'Tus reuniones', 'Deine Meetings'],
	'history.room': ['Room', 'Salle', 'Sala', 'Raum'],
	'history.participants': ['Participants', 'Participants', 'Participantes', 'Teilnehmende'],
	'history.transcript': ['Transcript', 'Transcription', 'Transcripción', 'Transkript'],
	'history.owner': ['Owner', 'Propriétaire', 'Propietario', 'Eigentümer'],
	'participants.guest': ['Guest', 'Invité', 'Invitado', 'Gast'],
	'participants.erased': ['Erased account', 'Compte effacé', 'Cuenta borrada', 'Gelöschtes Konto'],
	'recordings.failed': ['Recording unavailable', 'Enregistrement indisponible', 'Grabación no disponible', 'Aufzeichnung nicht verfügbar'],
	'recordings.player': ['Call recording', 'Enregistrement de l’appel', 'Grabación de la llamada', 'Aufzeichnung des Anrufs'],
	'recordings.shared': ['Shared with participants', 'Partagé avec les participants', 'Compartida con los participantes', 'Mit Teilnehmenden geteilt'],
	'recordings.private': ['Only you', 'Vous seul', 'Solo tú', 'Nur du'],
	'recordings.share': ['Share with participants', 'Partager avec les participants', 'Compartir con los participantes', 'Mit Teilnehmenden teilen'],
	'recordings.shareHint': [
		'Only the people who were in this call can watch it, signed in. Turn it off to take it back.',
		'Seules les personnes présentes à cet appel peuvent la voir, une fois connectées. Désactivez pour la reprendre.',
		'Solo las personas que estuvieron en esta llamada pueden verla, con sesión iniciada. Desactívalo para retirarla.',
		'Nur Personen, die an diesem Anruf teilgenommen haben, können sie angemeldet ansehen. Schalte es aus, um sie zurückzuziehen.'
	],
	'transcript.search': ['Search the transcript', 'Rechercher dans la transcription', 'Buscar en la transcripción', 'Im Transkript suchen'],
	'transcript.downloadTxt': ['Download .txt', 'Télécharger en .txt', 'Descargar .txt', '.txt herunterladen'],
	'transcript.downloadVtt': ['Download .vtt', 'Télécharger en .vtt', 'Descargar .vtt', '.vtt herunterladen'],
	'transcript.failed': ['Transcript unavailable', 'Transcription indisponible', 'Transcripción no disponible', 'Transkript nicht verfügbar'],
	'transcript.noMatch': ['No line matches.', 'Aucune ligne ne correspond.', 'Ninguna línea coincide.', 'Keine Zeile passt.'],
	'transcript.matches': [
		'{count} of {total} lines',
		'{count} lignes sur {total}',
		'{count} de {total} líneas',
		'{count} von {total} Zeilen'
	],
	'transcript.unknownSpeaker': ['Unknown speaker', 'Intervenant inconnu', 'Interlocutor desconocido', 'Unbekannte Person'],

	'plan.noAi': [
		'Transcripts and summaries are AI features, included with a paid plan.',
		'Les transcriptions et les résumés sont des fonctions IA, incluses dans un forfait payant.',
		'Las transcripciones y los resúmenes son funciones de IA, incluidas en un plan de pago.',
		'Transkripte und Zusammenfassungen sind KI-Funktionen und im kostenpflichtigen Tarif enthalten.'
	],
	'capture.askTitle': ['Your consent is asked', 'Votre accord est demandé', 'Se pide tu consentimiento', 'Deine Zustimmung wird erbeten'],
	'capture.askTranscribe': [
		'The organizer turned on transcription. Your microphone is turned into text on this device, and only the text is sent. No audio leaves your browser for this.',
		'L’organisateur a activé la transcription. Votre micro est converti en texte sur cet appareil, et seul le texte est envoyé. Aucun son ne quitte votre navigateur pour cela.',
		'El organizador activó la transcripción. Tu micrófono se convierte en texto en este dispositivo y solo se envía el texto. Ningún audio sale de tu navegador para ello.',
		'Der Organisator hat die Transkription eingeschaltet. Dein Mikrofon wird auf diesem Gerät in Text umgewandelt, und nur der Text wird gesendet. Dafür verlässt kein Ton deinen Browser.'
	],
	'capture.askRecord': [
		'The organizer turned on recording. Your voice would be added to an audio recording that only the organizer keeps. Video is never recorded.',
		'L’organisateur a activé l’enregistrement. Votre voix serait ajoutée à un enregistrement audio que seul l’organisateur conserve. La vidéo n’est jamais enregistrée.',
		'El organizador activó la grabación. Tu voz se añadiría a una grabación de audio que solo conserva el organizador. El vídeo nunca se graba.',
		'Der Organisator hat die Aufzeichnung eingeschaltet. Deine Stimme würde einer Audioaufzeichnung hinzugefügt, die nur der Organisator behält. Video wird nie aufgezeichnet.'
	],
	'capture.language': ['Speech language', 'Langue parlée', 'Idioma hablado', 'Gesprochene Sprache'],
	'capture.accept': ['Accept', 'Accepter', 'Aceptar', 'Zustimmen'],
	'capture.decline': ['Decline', 'Refuser', 'Rechazar', 'Ablehnen'],
	'capture.withdraw': ['Stop for me', 'Arrêter pour moi', 'Parar para mí', 'Für mich beenden'],
	'capture.transcribing': ['Transcription is on', 'La transcription est active', 'La transcripción está activada', 'Die Transkription ist eingeschaltet'],
	'capture.recording': ['Recording is on', 'L’enregistrement est actif', 'La grabación está activada', 'Die Aufzeichnung ist eingeschaltet'],
	'capture.youIn': ['You are included', 'Vous êtes inclus', 'Estás incluido', 'Du bist dabei'],
	'capture.youOut': ['You are not included', 'Vous n’êtes pas inclus', 'No estás incluido', 'Du bist nicht dabei'],
	'capture.leftOut': [
		{ one: '{count} person is not in the recording', other: '{count} people are not in the recording' },
		{ one: '{count} personne n’est pas dans l’enregistrement', other: '{count} personnes ne sont pas dans l’enregistrement' },
		{ one: '{count} persona no está en la grabación', other: '{count} personas no están en la grabación' },
		{ one: '{count} Person ist nicht in der Aufzeichnung', other: '{count} Personen sind nicht in der Aufzeichnung' }
	],
	'capture.downloading': [
		'Downloading the speech model ({percent}%). It is kept on this device for next time.',
		'Téléchargement du modèle vocal ({percent} %). Il reste sur cet appareil pour la prochaine fois.',
		'Descargando el modelo de voz ({percent} %). Se conserva en este dispositivo para la próxima vez.',
		'Sprachmodell wird geladen ({percent} %). Es bleibt für das nächste Mal auf diesem Gerät.'
	],
	'capture.starting': ['Starting speech recognition…', 'Démarrage de la reconnaissance vocale…', 'Iniciando el reconocimiento de voz…', 'Spracherkennung wird gestartet …'],
	'capture.modelFailed': [
		'Speech recognition could not start on this device. You are not transcribed.',
		'La reconnaissance vocale n’a pas pu démarrer sur cet appareil. Vous n’êtes pas transcrit.',
		'El reconocimiento de voz no pudo iniciarse en este dispositivo. No se te transcribe.',
		'Die Spracherkennung konnte auf diesem Gerät nicht starten. Du wirst nicht transkribiert.'
	],
	'capture.startTranscription': ['Start transcription', 'Démarrer la transcription', 'Iniciar la transcripción', 'Transkription starten'],
	'capture.stopTranscription': ['Stop transcription', 'Arrêter la transcription', 'Detener la transcripción', 'Transkription beenden'],
	'capture.startRecording': ['Start recording', 'Démarrer l’enregistrement', 'Iniciar la grabación', 'Aufzeichnung starten'],
	'capture.stopRecording': ['Stop recording', 'Arrêter l’enregistrement', 'Detener la grabación', 'Aufzeichnung beenden'],
	'capture.failed': ['Could not change this', 'Impossible de modifier ce réglage', 'No se pudo cambiar esto', 'Das ließ sich nicht ändern'],
	'capture.cannotRecord': [
		'This browser cannot record audio.',
		'Ce navigateur ne peut pas enregistrer le son.',
		'Este navegador no puede grabar audio.',
		'Dieser Browser kann keinen Ton aufzeichnen.'
	],
	'capture.uploadFailed': [
		'The recording upload failed. What was already saved is kept.',
		'Le transfert de l’enregistrement a échoué. Ce qui était déjà sauvegardé est conservé.',
		'Falló la subida de la grabación. Lo que ya se guardó se conserva.',
		'Das Hochladen der Aufzeichnung ist fehlgeschlagen. Das bereits Gespeicherte bleibt erhalten.'
	],
	'api.failed': ['request failed ({status})', 'la requête a échoué ({status})', 'la solicitud falló ({status})', 'Anfrage fehlgeschlagen ({status})'],
	'format.inProgress': ['In progress', 'En cours', 'En curso', 'Läuft'],

	'title.pending': ['Confirm · Echo', 'Confirmer · Echo', 'Confirmar · Echo', 'Bestätigen · Echo'],
	'pending.title': ['Waiting for you', 'En attente de vous', 'Esperando tu respuesta', 'Wartet auf dich'],
	'pending.description': [
		'An assistant connected to Echo asked to do this. Nothing happens until you confirm.',
		'Un assistant connecté à Echo a demandé à faire ceci. Rien ne se passe tant que vous ne confirmez pas.',
		'Un asistente conectado a Echo pidió hacer esto. No pasa nada hasta que confirmes.',
		'Ein mit Echo verbundener Assistent möchte das tun. Nichts passiert, bevor du bestätigst.'
	],
	'pending.signInTitle': ['Sign in to confirm', 'Connectez-vous pour confirmer', 'Inicia sesión para confirmar', 'Melde dich zum Bestätigen an'],
	'pending.signInBody': [
		'Sign in, then open the link from your assistant again.',
		'Connectez-vous, puis rouvrez le lien de votre assistant.',
		'Inicia sesión y vuelve a abrir el enlace de tu asistente.',
		'Melde dich an und öffne dann den Link deines Assistenten erneut.'
	],
	'pending.loadFailed': ['Could not load the requests', 'Impossible de charger les demandes', 'No se pudieron cargar las solicitudes', 'Anfragen konnten nicht geladen werden'],
	'pending.emptyTitle': ['Nothing to confirm', 'Rien à confirmer', 'Nada que confirmar', 'Nichts zu bestätigen'],
	'pending.emptyBody': [
		'Requests you already answered, and requests older than 7 days, are gone.',
		'Les demandes déjà traitées et celles de plus de 7 jours ont disparu.',
		'Las solicitudes ya respondidas y las de más de 7 días ya no están.',
		'Bereits beantwortete und über 7 Tage alte Anfragen sind verschwunden.'
	],
	'pending.deleteRoom': ['Delete room', 'Supprimer la salle', 'Eliminar sala', 'Raum löschen'],
	'pending.askedBy': ['Asked by {client} · {when}', 'Demandé par {client} · {when}', 'Pedido por {client} · {when}', 'Angefragt von {client} · {when}'],
	'pending.meetings': [
		{ one: '{count} meeting', other: '{count} meetings' },
		{ one: '{count} réunion', other: '{count} réunions' },
		{ one: '{count} reunión', other: '{count} reuniones' },
		{ one: '{count} Meeting', other: '{count} Meetings' }
	],
	'pending.deleteNote': [
		'The room and every meeting in it go for good: transcripts, summaries and recordings.',
		'La salle et toutes ses réunions disparaissent définitivement : transcriptions, résumés et enregistrements.',
		'La sala y todas sus reuniones se eliminan para siempre: transcripciones, resúmenes y grabaciones.',
		'Der Raum und alle seine Meetings werden endgültig gelöscht: Transkripte, Zusammenfassungen und Aufnahmen.'
	],
	'pending.changed': [
		'This room changed after the request, so it cannot be confirmed. Ask your assistant again.',
		'Cette salle a changé depuis la demande, elle ne peut donc pas être confirmée. Redemandez à votre assistant.',
		'Esta sala cambió después de la solicitud, así que no se puede confirmar. Vuelve a pedírselo a tu asistente.',
		'Dieser Raum hat sich seit der Anfrage geändert und kann nicht bestätigt werden. Frag deinen Assistenten erneut.'
	],
	'pending.missing': ['This room no longer exists.', 'Cette salle n’existe plus.', 'Esta sala ya no existe.', 'Dieser Raum existiert nicht mehr.'],
	'pending.confirm': ['Delete for good', 'Supprimer définitivement', 'Eliminar para siempre', 'Endgültig löschen'],
	'pending.dismiss': ['Dismiss', 'Ignorer', 'Descartar', 'Verwerfen'],
	'pending.deleted': ['The room is deleted.', 'La salle est supprimée.', 'La sala se eliminó.', 'Der Raum ist gelöscht.'],
	'pending.dismissed': ['Dismissed. Nothing was deleted.', 'Ignorée. Rien n’a été supprimé.', 'Descartada. No se eliminó nada.', 'Verworfen. Nichts wurde gelöscht.']
} as const satisfies Record<string, Row>;

export type MessageKey = keyof typeof rows;

type Column = Record<MessageKey, Message>;

function column(index: 0 | 1 | 2 | 3): Column {
	const entries = Object.entries(rows) as [MessageKey, Row][];
	return Object.fromEntries(entries.map(([key, row]) => [key, row[index]])) as Column;
}

export const catalogs: Catalogs<Column> = {
	en: column(0),
	fr: column(1),
	es: column(2),
	de: column(3)
};
