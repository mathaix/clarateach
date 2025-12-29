import { useState } from 'react';
import { LandingPage } from './components/LandingPage';
import { TeacherDashboard } from './components/TeacherDashboard';
import { TeacherClassView } from './components/TeacherClassView';
import { LearnerJoin } from './components/LearnerJoin';
import { Workspace } from './components/Workspace';

type View = 
  | { type: 'landing' }
  | { type: 'teacher-dashboard' }
  | { type: 'teacher-class'; classId: string; className: string; classCode: string }
  | { type: 'learner-join' }
  | { type: 'workspace'; learnerName: string; classCode: string };

export default function App() {
  const [currentView, setCurrentView] = useState<View>({ type: 'landing' });

  // Navigation handlers
  const handleSelectRole = (role: 'teacher' | 'learner') => {
    if (role === 'teacher') {
      setCurrentView({ type: 'teacher-dashboard' });
    } else {
      setCurrentView({ type: 'learner-join' });
    }
  };

  const handleStartClass = (classData: { name: string; maxSeats: number }) => {
    // Generate a random class code
    const code = Math.random().toString(36).substring(2, 8).toUpperCase();
    setCurrentView({
      type: 'teacher-class',
      classId: Date.now().toString(),
      className: classData.name,
      classCode: code
    });
  };

  const handleViewClass = (classId: string) => {
    setCurrentView({
      type: 'teacher-class',
      classId,
      className: 'Web Development 101',
      classCode: 'ABC123'
    });
  };

  const handleBackToDashboard = () => {
    setCurrentView({ type: 'teacher-dashboard' });
  };

  const handleEndClass = () => {
    setCurrentView({ type: 'teacher-dashboard' });
  };

  const handleBackToLanding = () => {
    setCurrentView({ type: 'landing' });
  };

  const handleLearnerJoin = (code: string, name: string) => {
    // In a real app, validate the code here
    setCurrentView({
      type: 'workspace',
      learnerName: name,
      classCode: code
    });
  };

  const handleLeaveWorkspace = () => {
    setCurrentView({ type: 'landing' });
  };

  // Render current view
  switch (currentView.type) {
    case 'landing':
      return <LandingPage onSelectRole={handleSelectRole} />;

    case 'teacher-dashboard':
      return (
        <TeacherDashboard
          onStartClass={handleStartClass}
          onViewClass={handleViewClass}
        />
      );

    case 'teacher-class':
      return (
        <TeacherClassView
          classCode={currentView.classCode}
          className={currentView.className}
          onBack={handleBackToDashboard}
          onEndClass={handleEndClass}
        />
      );

    case 'learner-join':
      return (
        <LearnerJoin
          onJoin={handleLearnerJoin}
          onBack={handleBackToLanding}
        />
      );

    case 'workspace':
      return (
        <Workspace
          learnerName={currentView.learnerName}
          classCode={currentView.classCode}
          onLeave={handleLeaveWorkspace}
        />
      );

    default:
      return <LandingPage onSelectRole={handleSelectRole} />;
  }
}
