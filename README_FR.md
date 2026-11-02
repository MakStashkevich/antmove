<div align="center">
  <h1>antmove</h1>
  <p><strong>déplace les apps et les données volumineuses sur un disque externe, et continue à utiliser tes apps comme avant.</strong></p>
  <p>plus de liberté face aux limites du stockage interne du Mac.</p>
  <p>macOS uniquement · CLI dans le Terminal · vérification des disques · aperçu avant transfert · restauration</p>
  <p><a href="README.md">🇺🇸 English</a> · <a href="README_JA.md">🇯🇵 日本語</a> · <a href="README_ZH_CN.md">🇨🇳 简体中文</a> · <a href="README_KO.md">🇰🇷 한국어</a> · <a href="README_FR.md">🇫🇷 Français</a> · <a href="README_UK.md">🇺🇦 Українська</a> · <a href="README_RU.md">🇷🇺 Русский</a></p>
  <p><a href="https://github.com/MakStashkevich/antmove/stargazers"><img src="https://img.shields.io/github/stars/MakStashkevich/antmove?style=flat" alt="Stars"></a> <a href="https://github.com/MakStashkevich/antmove/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a> <a href="https://github.com/MakStashkevich/antmove/releases/latest"><img src="https://img.shields.io/github/v/release/MakStashkevich/antmove?display_name=tag" alt="Latest release"></a> <a href="https://x.com/makstashkevich"><img src="https://img.shields.io/badge/X-follow-000000?logo=x&logoColor=white" alt="Follow on X"></a> <a href="https://t.me/maksidea"><img src="https://img.shields.io/badge/Telegram-join-26A5E4?logo=telegram&logoColor=white" alt="Telegram"></a></p>
</div>

l’espace du Mac se termine toujours au mauvais moment : quand il faut installer une app, mettre Xcode à jour, télécharger un jeu ou garder une sauvegarde de l’iPhone.

je suis passé par là. le SSD interne était rempli d’apps, de leurs données, d’outils de développement, de sauvegardes, de photos, de vidéos et de musique. à un moment, ça a commencé à gêner le travail tout court.

changer de Mac juste pour avoir un disque plus grand, ça me semblait être une réponse beaucoup trop chère. et pendant que les géants de l’IA construisent leurs centres de données, la mémoire et le stockage deviennent encore plus sensibles côté prix. moi, je n’avais pas besoin d’un nouvel ordinateur. j’avais besoin de place.

c’est comme ça qu’est né antmove. il déplace les apps et les données prises en charge vers un disque externe, vérifie la copie et laisse un lien symbolique à l’ancien emplacement. l’app continue d’aller à la même adresse. les gros fichiers, eux, sont maintenant sur un autre disque.

## concrètement, ça fait quoi

imagine que l’app arrive à son adresse habituelle et trouve là un panneau qui l’envoie vers le disque externe.

macOS et l’app gardent le chemin qu’ils connaissent. les fichiers lourds sont physiquement sur le disque externe connecté. voilà le principe.

on peut ajouter des disques compatibles au fur et à mesure. « stockage infini », c’est une image, hein. il faut toujours connecter le disque pour utiliser ce qu’il contient, et ne déplacer que ce qu’antmove sait reconnaître et vérifier.

## ce qu’antmove peut trouver

- les apps dans <code>/Applications</code> et <code>~/Applications</code> ;
- les gros dossiers de données qu’on peut clairement associer à une app installée ;
- les données Xcode et Apple Developer, Android SDK/AVD et certaines données d’outils de développement ;
- les photothèques Photos, les médias Apple Music, les anciens dossiers iTunes s’ils existent, les dossiers média/téléchargements Apple TV et les sauvegardes iPhone/iPad si macOS autorise l’accès ;
- les anciennes opérations Extender.

Docker Desktop, OrbStack, Colima et Rancher Desktop peuvent apparaître avec les conseils de leur éditeur. par contre antmove ne déplace pas directement leurs stockages internes. il y a des machines virtuelles et des bases de données là-dedans ; déplacer un dossier à la main peut casser le moteur.

si l’app propriétaire d’un grand dossier n’est pas claire, antmove ne fait pas semblant de savoir. il le laisse à part pour que tu regardes.

## installation

le nom de l’archive indique le Mac compatible : <code>apple-silicon</code> pour les Mac avec puce M, <code>intel</code> pour les Mac Intel. télécharge l’archive adaptée dans [GitHub Releases](https://github.com/MakStashkevich/antmove/releases).

décompresse l’archive et lance-la une première fois :

~~~sh
cd ~/Downloads/antmove
./antmove
~~~

au premier démarrage, antmove installe <code>antmove</code> et la commande courte <code>am</code> dans <code>~/.local/bin</code>. si ce dossier n’est pas dans <code>PATH</code>, il l’ajoute aux réglages du shell. après, rouvre Terminal.

tu peux aussi ne rien installer et lancer <code>./antmove</code> depuis le dossier décompressé.

pour installer ou réparer explicitement les commandes :

~~~sh
./install-local.sh
~~~

### installer avec le script Bash

<code>install.sh</code> repère macOS et l’architecture, télécharge la dernière version compatible et installe antmove. il ne scanne et ne déplace aucun fichier.

une seule commande suffit :

~~~sh
curl -fsSL https://raw.githubusercontent.com/MakStashkevich/antmove/main/install.sh | bash
~~~


le script est en Bash, tu peux le lire à la même adresse avant de le lancer. il télécharge seulement antmove et l’installe ; il ne scanne ni ne déplace tes fichiers.

### installer avec Homebrew

~~~sh
brew tap MakStashkevich/antmove https://github.com/MakStashkevich/antmove.git
brew install antmove
~~~

Pour mettre à jour ensuite :

~~~sh
brew upgrade antmove
~~~

Homebrew compile antmove depuis les sources et installe <code>antmove</code> ainsi que <code>am</code>. Go est installé automatiquement comme dépendance de compilation.

### compiler soi-même

il faut Go 1.24 ou plus récent :

~~~sh
git clone https://github.com/MakStashkevich/antmove.git
cd antmove
make build
./bin/antmove install
~~~

les binaires précompilés ne demandent ni Go, ni Swift, ni Xcode.

### demander à un assistant IA

le dépôt contient un skill antmove pour Codex, Claude Code, Cursor et les assistants du même genre :

~~~sh
npx skills add MakStashkevich/antmove -a codex claude-code cursor -g -y
~~~

les plugins Codex et Claude Code sont aussi prêts : <code>.agents/plugins</code> et <code>plugins/antmove</code>. les commandes sont dans <code>plugins/antmove/README.md</code>.

tu peux demander simplement :

> installe antmove avec les instructions officielles. explique les commandes, installe seulement l’outil et vérifie avec am --version. ne scanne et ne déplace rien tant que je ne l’ai pas demandé.

installer l’outil ne veut pas dire autoriser le déplacement des données. avant toute vraie opération, demande un aperçu <code>--dry-run</code>, regarde ce qui va changer, puis décide.

## par quoi commencer

ouvre le menu :

~~~sh
am
~~~

flèches pour bouger, Entrée pour choisir, Espace pour marquer. les raccourcis restent en bas de l’écran.

l’interface du terminal est en anglais. la documentation existe en sept langues.

pour commencer, regarde juste ce qu’il a trouvé :

~~~sh
am scan
~~~

puis les disques disponibles :

~~~sh
am disks
~~~

regarde le format, la vitesse, le type de connexion et l’espace libre. APFS sur SSD, c’est le choix préféré. HFS+ et APFS sur disque dur peuvent marcher, mais les apps et les jeux seront plus lents. ExFAT, FAT et les formats inconnus affichent un avertissement : les métadonnées macOS ou la compatibilité peuvent poser problème.

les disques réseau, systèmes, en lecture seule ou absents ne peuvent pas servir de destination.

pour mesurer la vitesse, antmove écrit un fichier temporaire de 256 Mio, le lit, puis le supprime :

~~~sh
am disks --test "/Volumes/Work SSD"
~~~

repères de vitesse en lecture :

| usage | suffisant | bien | très bien |
| --- | ---: | ---: | ---: |
| jeux | 150 MB/s | 350 MB/s | 700 MB/s |
| apps de bureau | 30 MB/s | 75 MB/s | 150 MB/s |
| messageries | 20 MB/s | 50 MB/s | 100 MB/s |

## regarde d’abord, déplace après

voilà l’ordre que je suivrais :

1. connecter le disque et le vérifier avec <code>am disks</code> ;
2. lancer <code>am scan</code> et regarder nom, taille, statut et note ;
3. quitter l’app concernée et attendre la fin de la sauvegarde ou de la synchro ;
4. choisir **Move data**, marquer les éléments, sélectionner un disque, lire l’avertissement et confirmer ;
5. garder le disque connecté pendant l’utilisation des données déplacées.

si une app liée est ouverte, le menu demande séparément avant d’envoyer une demande de fermeture normale ; le CLI propose <code>--close-apps</code>. macOS peut demander au terminal l’autorisation Automation. antmove ne force pas la fermeture et n’arrête pas les moteurs de conteneurs ni les services en arrière-plan.

tu peux prévisualiser avec l’ID de <code>am scan</code> :

~~~sh
am move --disk "/Volumes/Work SSD" --dry-run 71a29f5a1cd0
~~~

ça ne copie et ne change rien. si le plan te va, lance le vrai transfert :

~~~sh
am move --disk "/Volumes/Work SSD" --yes 71a29f5a1cd0
~~~

<code>--yes</code> confirme un transfert non interactif vers un disque recommandé. pour un disque avec avertissement, il faut confirmer deux fois :

~~~sh
am move --disk "/Volumes/Shared" --confirm-caution --confirm-caution 71a29f5a1cd0
~~~

ne confirme pas machinalement. lis pourquoi le disque est déconseillé. s’il est lent, les jeux peuvent ramer ; son système de fichiers peut aussi perdre des métadonnées macOS. deux confirmations ne le transforment pas en disque recommandé.

antmove vérifie l’espace libre, copie avec <code>ditto</code> dans un dossier temporaire, compare les empreintes et le contenu, puis crée le lien à l’ancien emplacement. le journal se trouve dans <code>~/Library/Application Support/antmove</code>.

## pour Photos, Music, les films et les sauvegardes

- **Photos :** quitte Photos avant de déplacer une photothèque <code>.photoslibrary</code>. Apple recommande APFS ou Mac OS étendu. Ouvre ensuite la photothèque depuis son nouvel emplacement ; avec Photos iCloud, choisis-la comme photothèque système si nécessaire.
- **Music :** quitte Music. Après le transfert, regarde **Music → Settings → Files → Media Location**, surtout si tu comptes importer de nouveaux fichiers. antmove ne change pas les réglages de Music.
- **Apple TV :** quitte TV aussi. Les films téléchargés peuvent être protégés par DRM. vérifie qu’ils se lancent après le déplacement et garde le disque connecté.
- **Chemin Apple TV :** Antmove lit le dossier Media défini dans <code>TV → Settings → Files</code> (par défaut <code>~/Movies/TV/Media.localized</code>) sans déplacer le dossier parent. Apple ne documente pas de chemin stable pour les téléchargements protégés ; un cache connu n’apparaît que s’il existe.
- **Fermeture des apps actives :** si une app liée est ouverte, Antmove demande séparément avant de lui envoyer une demande de fermeture normale. Pour le CLI, ajoute <code>--close-apps</code>. macOS peut demander au terminal l’autorisation Automation. Antmove ne force jamais la fermeture, ne supprime pas le travail non enregistré et n’arrête pas les sauvegardes, synchronisations ou services de conteneurs.
- **Sauvegardes iPhone/iPad :** l’emplacement standard est <code>~/Library/Application Support/MobileSync/Backup</code>. attends la fin des sauvegardes et synchronisations Finder. si macOS bloque l’accès, donne l’accès complet au disque à l’app Terminal qui lance <code>am</code>, rouvre-la et rescane.
- **Dossiers vides ou pas encore créés :** <code>am scan</code> affiche aussi les emplacements standard de Music, Photos, Apple TV et des sauvegardes iPhone/iPad, même s’ils sont vides ou absents. un élément de <code>0 B</code> peut être déplacé : après confirmation, antmove crée le dossier d’origine si nécessaire et le relie au disque externe. les prochaines écritures faites à cet emplacement peuvent alors aller sur le disque externe. quitte l’app concernée et attends la fin des sauvegardes ou synchronisations. pour Music, vérifie que Media Location pointe vers le dossier lié. une photothèque Photos est un paquet géré par l’app : crée-en une nouvelle avec Photos ouvert en maintenant Option, puis Create New. si Photos ne peut pas initialiser le lien vide, crée la photothèque directement sur le disque externe et sélectionne-la dans Photos. les anciens chemins iTunes ne s’affichent que si les dossiers existent déjà.
- **Docker :** utilise la méthode de migration prévue par le moteur. déplacer à la main les fichiers d’une VM ou d’une base de données, je ne le ferais pas.

Guides Apple : [déplacer une photothèque](https://support.apple.com/en-ie/108345) · [fichiers Music](https://support.apple.com/en-ca/guide/music/mus69248042d/mac) · [réglages Apple TV](https://support.apple.com/guide/tvapp/files-settings-atvb5536de97/web) · [retrouver les sauvegardes iPhone](https://support.apple.com/en-la/108809).

## si tu veux remettre les données à leur place

regarde d’abord le plan :

~~~sh
am restore --dry-run 71a29f5a1cd0
~~~

si ça te convient, restaure :

~~~sh
am restore --yes 71a29f5a1cd0
~~~

<code>am status</code> affiche le journal et l’état du disque. si un transfert a été interrompu, reconnecte le disque et lance <code>am recover</code>.

pour désinstaller antmove, il faut confirmer deux fois :

~~~sh
am uninstall
~~~

la désinstallation ne remet pas les données à leur place. utilise <code>am restore</code> avant, ou garde le disque connecté et les liens en place.

## toutes les commandes

| commande | rôle |
| --- | --- |
| <code>am</code> | ouvrir le menu |
| <code>am install</code> | installer ou réparer les commandes |
| <code>am uninstall</code> | supprimer après deux confirmations |
| <code>am scan</code> | trouver les candidats et leur statut |
| <code>am disks</code> | afficher les disques utilisables |
| <code>am disks --test PATH</code> | mesurer lecture et écriture |
| <code>am move --disk PATH [ID…]</code> | déplacer les éléments choisis |
| <code>am restore [ID…]</code> | restaurer à l’emplacement d’origine |
| <code>am status</code> | afficher le journal et l’état des disques |
| <code>am recover</code> | reprendre après une interruption |
| <code>am migrate --dry-run</code> | prévisualiser l’import Extender |
| <code>am migrate</code> | importer les anciennes opérations Extender |
| <code>am --help</code> | afficher l’aide |
| <code>am --version</code> | afficher la version et le système |

## si tu veux compiler le projet

il faut Go 1.24 ou plus récent. sur macOS, tu peux compiler pour ta machine ou préparer les deux architectures :

~~~sh
make build
make build-arm64
make build-amd64
make release
make test
make check
~~~

les archives arrivent dans <code>dist/</code>. les binaires ne sont pas encore signés ou notariés. Gatekeeper peut demander une autorisation dans **Réglages Système → Confidentialité et sécurité**.

## licence

antmove est distribué sous la [licence MIT](LICENSE).

