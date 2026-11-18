# Kanso

Petite app locale pour suivre les dépenses pro : paiements à venir, factures déjà rangées, factures à
récupérer. Elle lit et écrit directement dans `~/Documents/Inc`.

```bash
make run          # compile puis lance http://localhost:48219
```

## Ce qu'elle fait

| Vue | Contenu |
| --- | --- |
| Calendrier | les échéances de tes abonnements et paiements prévus, les factures du dossier, le total du mois |
| Analyse | les dépenses en euros par mois, par projet et par service ; chaque service ou facture se classe dans un projet (ou tout seul, par un mot de son nom) |
| Factures | toutes les factures lues dans `Inc`, par mois ; facture + reçu (ou commande + relevé) = une seule ligne |
| Démarches | déclarations URSSAF (chaque trimestre ou chaque mois), CFE (15 décembre), déclaration de revenus (fin mai) : dans le calendrier, à cocher une fois faites |
| À récupérer | chaque échéance passée sans facture ; dépose le PDF sur la carte, il est rangé dans `Année/Fournisseur/Mois` |
| Dossier Inc | le dossier tel quel : créer un dossier, déposer, renommer, déplacer (glisser sur un dossier) |

- **Le dossier est la vérité.** Ce que tu ajoutes dans le Finder apparaît dans l'app en 4 s, et inversement.
- **Lecture des PDF** : `bin/pdftext`, un mini outil Swift basé sur PDFKit (fourni par macOS). Montant, date,
  numéro et fournisseur sont lus une fois par fichier, puis gardés en cache.
- **Ce que le dossier ne dit pas** (abonnements, projets, pense-bête, corrections) : `Inc/.kanso/db.json` (et tes fournisseurs perso dans `Inc/.kanso/vendors.json`).
- **Cache** (texte des PDF, logos) : `~/Library/Caches/kanso/` — supprimable sans perte.
- **Logos** : téléchargés une fois depuis le site du fournisseur (sinon le service d'icônes de Google), puis gardés en cache.

## Stack

Go, bibliothèque standard uniquement (aucune dépendance), front HTML/CSS/JS sans framework embarqué dans le
binaire. Écoute sur `127.0.0.1` seulement ; les écritures exigent l'en-tête `X-Inc` (un autre site ne peut pas
les déclencher), les chemins hors du dossier sont refusés.

## Lancement automatique

```bash
make install-agent      # une fois : l'app démarre avec le Mac, sans terminal
make build              # après une modification : l'app redémarre seule en 2-3 s, la page se recharge
make logs               # les dernières lignes du journal (~/Library/Logs/kanso.log)
make uninstall-agent    # retire le lancement automatique
```

launchd lance `bin/kanso-launcher`, un petit lanceur qui ne change pas d'une version à l'autre : il
démarre l'app, la relance si elle s'arrête, et la redémarre dès que `bin/kanso` est recompilé. La page
ouverte se recharge d'elle-même quand le serveur a redémarré. macOS ne demande l'accès au dossier Documents
qu'une fois, pour le lanceur.

## App Mac et rappels

```bash
make app                # construit « Kanso.app » et l'installe dans ~/Applications
```

Une fenêtre sur le serveur local (plus besoin d'onglet de navigateur) et des notifications macOS, programmées
dans le système : elles arrivent même app fermée. À 9 h : la veille d'un paiement, 7 jours puis 1 jour avant
une démarche, chaque lundi tant qu'il reste des factures à récupérer, et 30, 14, 7 et 1 jours avant que la
réserve tombe à zéro. L'app relit la liste au démarrage, toutes les 30 minutes et à chaque changement fait
dans sa fenêtre (`GET /api/reminders`). Menu « Kanso › Envoyer un rappel de test » pour vérifier que
macOS les laisse passer. Fermer la fenêtre garde l'app ouverte ; pour l'ouvrir avec le Mac : clic droit sur
son icône du Dock › Options › Ouvrir à l'ouverture de session.

